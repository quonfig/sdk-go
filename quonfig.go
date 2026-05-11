// Package quonfig provides a client for fetching configuration and feature flags from the Quonfig API.
package quonfig

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/quonfig/sdk-go/internal/version"
)

// ErrNotFound is returned when a config key does not exist.
var ErrNotFound = errors.New("config not found")

// ErrInitializationTimeout is returned when the client could not finish its initial fetch before the configured timeout.
var ErrInitializationTimeout = errors.New("initialization_timeout")

// ErrMissingEnvVar is returned when an ENV_VAR-provided config references a missing environment variable.
var ErrMissingEnvVar = errors.New("missing_env_var")

// ErrUnableToCoerce is returned when an ENV_VAR-provided config cannot be coerced to the target type.
var ErrUnableToCoerce = errors.New("unable_to_coerce_env_var")

// ErrUnableToDecrypt is returned when a confidential value cannot be decrypted.
var ErrUnableToDecrypt = errors.New("unable_to_decrypt")

// configStore is a minimal interface for looking up configs by key.
type configStore interface {
	Get(key string) (*ConfigResponse, bool)
	Keys() []string
}

// ConfigEvaluator evaluates a config against a context.
// This interface breaks the import cycle between quonfig and internal/eval.
type ConfigEvaluator interface {
	// EvaluateConfigResponse evaluates a ConfigResponse for the given environment and context.
	// Returns the full evaluation result including match metadata for telemetry and reasons.
	EvaluateConfigResponse(cfg *ConfigResponse, envID string, ctx *ContextSet) *EvalResult
}

// ValueResolver resolves a matched value (e.g., ENV_VAR lookup, decryption).
type ValueResolver interface {
	// ResolveValue resolves a matched value, handling ENV_VAR provided values and decryption.
	// The configKey and valueType are used for coercion and error messages.
	ResolveValue(val *Value, configKey string, valueType ValueType, envID string, ctx *ContextSet) (*Value, error)
}

// Client is the main Quonfig SDK client.
type Client struct {
	opts      Options
	store     configStore
	evaluator ConfigEvaluator
	resolver  ValueResolver
	envID     string // environment ID for evaluation (e.g. "Production")

	transport *runtimeTransport
	telemetry *telemetrySubmitter
	sse       *sseClient

	mu                     sync.RWMutex
	initializationDone     chan struct{}
	initializationStarted  bool
	initializationTimedOut bool
	initialized            bool
	initializationErr      error
	refreshMu              sync.Mutex
	closeCh                chan struct{}
	closeOnce              sync.Once
}

// NewClient creates a new Quonfig client with the given options.
// If an API key is configured, the client begins an initial config download and
// wires local evaluation automatically. Background refresh is opt-in via WithRefreshInterval.
func NewClient(opts ...Option) (*Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	// Env var overrides apply after explicit options. Explicit With*
	// options always win — the override functions check the *explicit
	// flags and skip fields the caller already set.
	applyDomainEnvOverride(&o)
	applyEnvironmentEnvOverride(&o)
	applyAPIKeyEnvOverride(&o)
	applyDevContextEnvOverride(&o)

	if o.Logger == nil {
		o.Logger = slog.Default()
	}

	if o.EnableQuonfigUserContext {
		if devCtx := loadQuonfigUserContext(o.APIURLs, o.Logger); devCtx != nil {
			// Customer-supplied GlobalContext wins on collision because
			// Merge replaces by named-context name and the second arg wins.
			o.GlobalContext = Merge(devCtx, o.GlobalContext)
		}
	}

	// Build transport before struct construction so c.transport is set
	// exactly once in the struct literal and is effectively immutable for
	// the lifetime of the Client. Readers (Refresh, fetchAndInstall,
	// awaitInitialization, startSSE) — including ones running in the init
	// goroutine — can therefore read the field without synchronization.
	var transport *runtimeTransport
	if o.DataDir == "" && o.APIKey != "" {
		transport = newRuntimeTransportWithStreamOverride(o.APIURLs, o.APIKey, o.HTTPClient, o.testStreamURLOverride)
	}

	client := &Client{
		opts:               o,
		transport:          transport,
		initializationDone: make(chan struct{}),
		closeCh:            make(chan struct{}),
	}

	if o.TelemetryEnabled() {
		client.telemetry = newTelemetrySubmitter(o)
		client.telemetry.Start()
	}

	if o.DataDir != "" {
		envelope, err := loadWorkspaceEnvelope(o.DataDir, o.Environment)
		if err != nil {
			if client.telemetry != nil {
				client.telemetry.Stop()
			}
			return nil, err
		}
		client.installEnvelope(envelope)
		client.finishInitialization(true)
		return client, nil
	}

	if o.APIKey == "" {
		client.initialized = true
		close(client.initializationDone)
		return client, nil
	}

	client.startInitialization()

	return client, nil
}

// setStore sets the config store (internal wiring).
func (c *Client) setStore(s configStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = s
}

// Refresh performs a manual poll of GET /api/v2/configs using ETag caching.
func (c *Client) Refresh() error {
	if c.transport == nil {
		return nil
	}
	return c.fetchAndInstall(context.Background(), false)
}

// Close stops any background refresh loop, shuts down the SSE stream, and
// flushes pending telemetry.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		// Read c.sse under the lock so we synchronize with startSSE, which
		// runs asynchronously from the init goroutine and may not yet have
		// installed the SSE client. If startSSE runs after Close, it sees
		// closeCh closed and skips installation.
		c.mu.Lock()
		sse := c.sse
		c.mu.Unlock()
		if sse != nil {
			sse.Stop()
		}
		if c.telemetry != nil {
			c.telemetry.Stop()
		}
	})
}

// GetStringValue returns the string value for a config key.
func (c *Client) GetStringValue(key string, ctx *ContextSet) (string, bool, error) {
	val, ok, err := c.resolve(key, ctx)
	if err != nil || !ok {
		return "", false, err
	}
	return val.StringValue(), true, nil
}

// GetIntValue returns the int64 value for a config key.
func (c *Client) GetIntValue(key string, ctx *ContextSet) (int64, bool, error) {
	val, ok, err := c.resolve(key, ctx)
	if err != nil || !ok {
		return 0, false, err
	}
	return val.IntValue(), true, nil
}

// GetBoolValue returns the bool value for a config key.
func (c *Client) GetBoolValue(key string, ctx *ContextSet) (bool, bool, error) {
	val, ok, err := c.resolve(key, ctx)
	if err != nil || !ok {
		return false, false, err
	}
	return val.BoolValue(), true, nil
}

// GetFloatValue returns the float64 value for a config key.
func (c *Client) GetFloatValue(key string, ctx *ContextSet) (float64, bool, error) {
	val, ok, err := c.resolve(key, ctx)
	if err != nil || !ok {
		return 0, false, err
	}
	return val.DoubleValue(), true, nil
}

// GetStringSliceValue returns the []string value for a config key.
func (c *Client) GetStringSliceValue(key string, ctx *ContextSet) ([]string, bool, error) {
	val, ok, err := c.resolve(key, ctx)
	if err != nil || !ok {
		return nil, false, err
	}
	return val.StringListValue(), true, nil
}

// GetDurationValue returns the time.Duration value for a config key.
// The stored value should be an ISO 8601 duration string (e.g., "PT90S", "PT1.5M", "P1DT6H2M1.5S").
func (c *Client) GetDurationValue(key string, ctx *ContextSet) (time.Duration, bool, error) {
	val, ok, err := c.resolve(key, ctx)
	if err != nil || !ok {
		return 0, false, err
	}
	s := val.StringValue()
	d, parseErr := ParseISO8601Duration(s)
	if parseErr != nil {
		return 0, true, fmt.Errorf("parsing duration %q: %w", s, parseErr)
	}
	return d, true, nil
}

// GetJSONValue returns the parsed JSON value for a config key.
// Values are stored natively (object/array/number/boolean/null); this is a
// direct pass-through of Value.Value.
func (c *Client) GetJSONValue(key string, ctx *ContextSet) (interface{}, bool, error) {
	val, ok, err := c.resolve(key, ctx)
	if err != nil || !ok {
		return nil, false, err
	}
	return val.Value, true, nil
}

// FeatureIsOn returns whether a feature flag is on. Returns false if the key is not found.
func (c *Client) FeatureIsOn(key string, ctx *ContextSet) (bool, bool) {
	val, ok, err := c.GetBoolValue(key, ctx)
	if err != nil || !ok {
		return false, false
	}
	return val, true
}

// WithContext returns a ContextBoundClient that merges the given context into every call.
func (c *Client) WithContext(ctx *ContextSet) *ContextBoundClient {
	merged := Merge(c.opts.GlobalContext, ctx)
	return &ContextBoundClient{client: c, ctx: merged}
}

// Keys returns all config keys currently in the store.
func (c *Client) Keys() []string {
	if err := c.awaitInitialization(""); err != nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return nil
	}
	return c.store.Keys()
}

// EvaluateKey resolves a config key and returns the resolved value, evaluation
// reason, and ok flag. Retained for backward compatibility; new code should
// prefer EvaluateDetails which returns the full EvaluationDetails record
// (typed ErrorCode, Variant, FlagMetadata).
func (c *Client) EvaluateKey(key string, ctx *ContextSet) (*Value, EvalReason, bool, error) {
	d, err := c.resolveDetail(key, ctx)
	found := err == nil && d.Value != nil
	reason := d.Reason
	// Preserve original semantics: not-found returns ReasonDefault (not
	// ReasonError) so existing callers (load-gen, OpenFeature provider mapping)
	// keep working unchanged.
	if errors.Is(err, ErrNotFound) {
		reason = ReasonDefault
	}
	return d.Value, reason, found, err
}

// EvaluateDetails resolves a config key and returns a full EvaluationDetails
// record including typed ErrorCode, ErrorMessage, Variant, and FlagMetadata.
// This is the recommended API for OpenFeature providers and any caller that
// needs structured evaluation metadata.
func (c *Client) EvaluateDetails(key string, ctx *ContextSet) EvaluationDetails {
	d, _ := c.resolveDetail(key, ctx)
	return d
}

// resolve looks up a config and evaluates it against the given context.
func (c *Client) resolve(key string, ctx *ContextSet) (*Value, bool, error) {
	d, err := c.resolveDetail(key, ctx)
	found := err == nil && d.Value != nil
	return d.Value, found, err
}

// resolveDetail returns both an EvaluationDetails record (the public-API view)
// and the underlying Go error (preserving error identity for errors.Is checks
// in EvaluateKey and other backward-compatible callers).
func (c *Client) resolveDetail(key string, ctx *ContextSet) (EvaluationDetails, error) {
	if err := c.awaitInitialization(key); err != nil {
		if c.opts.OnInitFailure == ReturnZeroValue && errors.Is(err, ErrInitializationTimeout) {
			return EvaluationDetails{
				Reason:       ReasonDefault,
				Variant:      "default",
				FlagMetadata: map[string]any{},
			}, nil
		}
		return EvaluationDetails{
			Reason:       ReasonError,
			ErrorCode:    errorCodeFor(err),
			ErrorMessage: err.Error(),
			Variant:      "default",
			FlagMetadata: map[string]any{},
		}, err
	}

	c.mu.RLock()
	store := c.store
	evaluator := c.evaluator
	resolver := c.resolver
	envID := c.envID
	globalContext := c.opts.GlobalContext
	telemetry := c.telemetry
	c.mu.RUnlock()

	notFound := func() (EvaluationDetails, error) {
		return EvaluationDetails{
			Reason:       ReasonError,
			ErrorCode:    ErrorCodeFlagNotFound,
			ErrorMessage: ErrNotFound.Error(),
			Variant:      "default",
			FlagMetadata: map[string]any{},
		}, ErrNotFound
	}

	if store == nil {
		return notFound()
	}
	cfg, ok := store.Get(key)
	if !ok {
		return notFound()
	}

	mergedCtx := Merge(globalContext, ctx)

	// Record context for telemetry (before evaluation, same as old sdk-go)
	if telemetry != nil {
		telemetry.RecordContext(mergedCtx)
	}

	// If we have an evaluator, use it for full rule evaluation with context.
	if evaluator != nil {
		evalResult := evaluator.EvaluateConfigResponse(cfg, envID, mergedCtx)

		// Record evaluation for telemetry
		if telemetry != nil && evalResult != nil {
			telemetry.RecordEvaluation(evalResult)
		}

		if evalResult == nil || !evalResult.IsMatch || evalResult.Value == nil {
			return EvaluationDetails{
				Reason:       ReasonDefault,
				Variant:      "default",
				FlagMetadata: flagMetadataForConfig(cfg, envID),
			}, nil
		}

		// Pass through the resolver if available (handles ENV_VAR, decryption).
		value := evalResult.Value
		if resolver != nil {
			resolved, err := resolver.ResolveValue(evalResult.Value, cfg.Key, cfg.ValueType, envID, mergedCtx)
			if err != nil {
				return EvaluationDetails{
					Reason:       ReasonError,
					ErrorCode:    errorCodeFor(err),
					ErrorMessage: err.Error(),
					Variant:      "default",
					FlagMetadata: flagMetadataFor(evalResult, envID),
				}, err
			}
			value = resolved
		}
		return EvaluationDetails{
			Value:        value,
			Reason:       evalResult.Reason,
			Variant:      variantFor(evalResult.Reason, evalResult.RuleIndex, evalResult.WeightedValueIndex),
			FlagMetadata: flagMetadataFor(evalResult, envID),
		}, nil
	}

	// Fallback: return the first default rule's value (no evaluator available).
	if len(cfg.Default.Rules) > 0 {
		val := cfg.Default.Rules[0].Value
		return EvaluationDetails{
			Value:        &val,
			Reason:       ReasonUnknown,
			Variant:      "default",
			FlagMetadata: flagMetadataForConfig(cfg, envID),
		}, nil
	}
	return EvaluationDetails{
		Reason:       ReasonDefault,
		Variant:      "default",
		FlagMetadata: flagMetadataForConfig(cfg, envID),
	}, nil
}

// flagMetadataForConfig builds flagMetadata from a ConfigResponse alone (no
// rule match). Used for the default / no-match path so consumers still see
// configId / configType / environment.
func flagMetadataForConfig(cfg *ConfigResponse, envID string) map[string]any {
	md := make(map[string]any)
	if cfg == nil {
		return md
	}
	if cfg.ID != "" {
		md["configId"] = cfg.ID
	}
	if upper := configTypeUpper(cfg.Type); upper != "" {
		md["configType"] = upper
	}
	if envID != "" {
		md["environment"] = envID
	}
	return md
}

// errorCodeFor maps a Go error to an OpenFeature-shaped ErrorCode. The mapping
// is keyed off sentinel errors (errors.Is) -- not message text -- so error
// message wording can change without breaking downstream provider semantics.
func errorCodeFor(err error) ErrorCode {
	switch {
	case err == nil:
		return ErrorCodeNone
	case errors.Is(err, ErrNotFound):
		return ErrorCodeFlagNotFound
	case errors.Is(err, ErrInitializationTimeout):
		return ErrorCodeProviderNotReady
	case errors.Is(err, ErrUnableToCoerce):
		return ErrorCodeTypeMismatch
	case errors.Is(err, ErrMissingEnvVar), errors.Is(err, ErrUnableToDecrypt):
		return ErrorCodeGeneral
	default:
		return ErrorCodeGeneral
	}
}

func (c *Client) startInitialization() {
	c.mu.Lock()
	if c.initializationStarted {
		c.mu.Unlock()
		return
	}
	c.initializationStarted = true
	c.mu.Unlock()

	go func() {
		ctx := context.Background()
		if c.opts.InitTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.opts.InitTimeout)
			defer cancel()
		}

		_ = c.fetchAndInstall(ctx, true)

		if c.opts.SSEEnabled {
			c.startSSE()
		}

		if c.opts.RefreshInterval > 0 {
			c.startRefreshLoop()
		}
	}()
}

// startSSE opens the long-lived SSE stream to streamURLFor(0) and installs
// received envelopes via the same path as HTTP polling. Called from the
// init goroutine so the initial HTTP fetch always wins the race on startup
// (a cold SDK with nothing in the store is the one case where we really
// need the fetch to land first; after that, either path can overwrite).
func (c *Client) startSSE() {
	if c.transport == nil {
		return
	}
	url := c.transport.streamURLFor(0)
	if url == "" {
		return
	}
	sse := newSSEClient(sseClientConfig{
		URL:       url,
		APIKey:    c.opts.APIKey,
		UserAgent: version.Header(),
		// Intentionally do not forward c.opts.HTTPClient: the SSE socket
		// needs HTTP/1.1 forced (an h2 stream stall is invisible in CI),
		// and the runtime read-deadline machinery lives in newSSEClient's
		// default transport. The polling path keeps using HTTPClient.
		ReadTimeout: c.opts.testSSEReadTimeout,
		Logger:      c.opts.Logger,
		OnEnvelope: func(env *ConfigEnvelope) {
			// Serialize with polled installs via refreshMu so we don't race.
			c.refreshMu.Lock()
			c.installEnvelope(env)
			c.refreshMu.Unlock()
		},
		OnStateChange: c.opts.OnSSEStateChange,
	})

	// If Close ran before we got here, don't install or start the SSE
	// client — Close has already finished and would not stop us.
	c.mu.Lock()
	select {
	case <-c.closeCh:
		c.mu.Unlock()
		return
	default:
	}
	c.sse = sse
	c.mu.Unlock()

	sse.Start()
}

func (c *Client) startRefreshLoop() {
	ticker := time.NewTicker(c.opts.RefreshInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = c.Refresh()
			case <-c.closeCh:
				return
			}
		}
	}()
}

func (c *Client) fetchAndInstall(ctx context.Context, initial bool) error {
	c.refreshMu.Lock()
	defer c.refreshMu.Unlock()

	result, err := c.transport.FetchConfigs(ctx)
	if err != nil {
		if initial {
			storedErr := err
			// Normalize a context.DeadlineExceeded that came from our own
			// init timeout into ErrInitializationTimeout. Otherwise the
			// caller's errors.Is(err, ErrInitializationTimeout) check is
			// non-deterministic: it succeeds when awaitInitialization's
			// timer fires first, but fails when the fetch context's
			// deadline fires first and stores the raw context error.
			if c.opts.InitTimeout > 0 && errors.Is(err, context.DeadlineExceeded) {
				storedErr = c.initializationTimeoutError("")
			}
			c.mu.Lock()
			c.initializationErr = storedErr
			c.mu.Unlock()
			c.finishInitialization(false)
		}
		return err
	}

	if result.NotChanged {
		if initial {
			c.mu.Lock()
			c.initialized = true
			c.initializationErr = nil
			c.mu.Unlock()
			c.finishInitialization(true)
		}
		return nil
	}

	c.installEnvelope(result.Envelope)

	if initial {
		c.finishInitialization(true)
	}
	return nil
}

func (c *Client) installEnvelope(envelope *ConfigEnvelope) {
	store := newRuntimeStore()
	store.Update(envelope)
	evaluator := newRuntimeEvaluator(store)
	resolver := newRuntimeResolver(store, evaluator, c.opts.EnvLookup)

	c.mu.Lock()
	c.store = store
	c.evaluator = evaluator
	c.resolver = resolver
	c.envID = envelope.Meta.Environment
	c.initialized = true
	c.initializationErr = nil
	onConfigUpdate := c.opts.OnConfigUpdate
	c.mu.Unlock()

	if onConfigUpdate != nil {
		onConfigUpdate()
	}
}

func (c *Client) finishInitialization(success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.initializationDone:
		return
	default:
		if !success && c.initializationErr == nil {
			c.initializationErr = ErrInitializationTimeout
		}
		close(c.initializationDone)
	}
}

func (c *Client) awaitInitialization(key string) error {
	c.mu.RLock()
	transport := c.transport
	initialized := c.initialized
	timedOut := c.initializationTimedOut
	initErr := c.initializationErr
	done := c.initializationDone
	timeout := c.opts.InitTimeout
	c.mu.RUnlock()

	if transport == nil || initialized {
		return nil
	}
	if initErr != nil && !timedOut {
		return initErr
	}
	if timedOut {
		return c.initializationTimeoutError(key)
	}

	var timeoutCh <-chan time.Time
	if timeout <= 0 {
		timer := time.NewTimer(0)
		defer timer.Stop()
		timeoutCh = timer.C
	} else {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	select {
	case <-done:
		c.mu.RLock()
		defer c.mu.RUnlock()
		if c.initialized {
			return nil
		}
		if c.initializationErr != nil {
			return c.initializationErr
		}
		return ErrInitializationTimeout
	case <-timeoutCh:
		c.mu.Lock()
		c.initializationTimedOut = true
		c.mu.Unlock()
		return c.initializationTimeoutError(key)
	}
}

func (c *Client) initializationTimeoutError(key string) error {
	if key == "" {
		return fmt.Errorf("%w: client initialization exceeded %s", ErrInitializationTimeout, c.opts.InitTimeout)
	}
	return fmt.Errorf("%w: client initialization exceeded %s while resolving %q", ErrInitializationTimeout, c.opts.InitTimeout, key)
}

// ContextBoundClient is a Client bound to a specific context.
type ContextBoundClient struct {
	client *Client
	ctx    *ContextSet
}

// GetStringValue returns the string value for a config key using the bound context.
func (cb *ContextBoundClient) GetStringValue(key string) (string, bool, error) {
	return cb.client.GetStringValue(key, cb.ctx)
}

// GetIntValue returns the int64 value for a config key using the bound context.
func (cb *ContextBoundClient) GetIntValue(key string) (int64, bool, error) {
	return cb.client.GetIntValue(key, cb.ctx)
}

// GetBoolValue returns the bool value for a config key using the bound context.
func (cb *ContextBoundClient) GetBoolValue(key string) (bool, bool, error) {
	return cb.client.GetBoolValue(key, cb.ctx)
}

// GetFloatValue returns the float64 value for a config key using the bound context.
func (cb *ContextBoundClient) GetFloatValue(key string) (float64, bool, error) {
	return cb.client.GetFloatValue(key, cb.ctx)
}

// GetStringSliceValue returns the []string value for a config key using the bound context.
func (cb *ContextBoundClient) GetStringSliceValue(key string) ([]string, bool, error) {
	return cb.client.GetStringSliceValue(key, cb.ctx)
}

// GetDurationValue returns the time.Duration value for a config key using the bound context.
func (cb *ContextBoundClient) GetDurationValue(key string) (time.Duration, bool, error) {
	return cb.client.GetDurationValue(key, cb.ctx)
}

// GetJSONValue returns the parsed JSON value for a config key using the bound context.
func (cb *ContextBoundClient) GetJSONValue(key string) (interface{}, bool, error) {
	return cb.client.GetJSONValue(key, cb.ctx)
}

// FeatureIsOn returns whether a feature flag is on using the bound context.
func (cb *ContextBoundClient) FeatureIsOn(key string) (bool, bool) {
	return cb.client.FeatureIsOn(key, cb.ctx)
}

// WithContext returns a new ContextBoundClient with the given context merged in.
func (cb *ContextBoundClient) WithContext(ctx *ContextSet) *ContextBoundClient {
	merged := Merge(cb.ctx, ctx)
	return &ContextBoundClient{client: cb.client, ctx: merged}
}
