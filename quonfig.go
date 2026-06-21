// Package quonfig provides a client for fetching configuration and feature flags from the Quonfig API.
package quonfig

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
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
	sup       *supervisor
	fallback  *fallbackPoller
	watcher   *datadirWatcher

	mu                     sync.RWMutex
	initializationDone     chan struct{}
	initializationStarted  bool
	initializationTimedOut bool
	initialized            bool
	initializationErr      error
	refreshMu              sync.Mutex
	closeCh                chan struct{}
	closeOnce              sync.Once

	// Failover / canonical-ordering observability (guarded by mu). These back
	// the public HeldGeneration/ResolvedFrom/ConfigInstallCount/
	// SSEFailedOverToSecondary accessors used by the failover + ordering chaos
	// suites; all additive, none change evaluation behavior.
	heldGeneration    int // Meta.Generation of the currently-installed envelope
	configInstalls    int // count of installEnvelope calls (every install path)
	resolvedFromIndex int // baseURLs index of the last HTTP fetch that installed; -1 until set
	sseStreamIndex    int // baseURLs index the SSE stream connected with; -1 until connected
}

// NewClient creates a new Quonfig client with the given options.
// If an API key is configured, the client begins an initial config download and
// wires local evaluation automatically. Background refresh is configured via
// WithFallbackPoll (default: enabled, 60s interval).
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

	if o.Logger == nil {
		o.Logger = slog.Default()
	}

	if resolveDevContextEnabled(&o) {
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
		// Per-leg config-fetch deadlines. Set once, before any goroutine reads the
		// transport, so they stay effectively immutable. fetchTimeout bounds the
		// sequential FetchConfigs path (qfg-7h5d.1.4); hedgeDelay/hedgeAbort bound
		// the parallel hedge that the init/refresh install path uses (qfg-7h5d.1.14).
		transport.fetchTimeout = o.ConfigFetchTimeout
		transport.hedgeDelay = o.ConfigFetchHedgeDelay
		transport.hedgeAbort = o.ConfigFetchHedgeAbort

		// The init-path heal leg runs under a context whose deadline is the
		// InitTimeout; if the per-leg hedge abort is >= InitTimeout the heal leg is
		// clipped and a late-but-newer primary can't heal forward. Warn rather than
		// hard-fail so a deliberately short InitTimeout still works (just without
		// init-time heal-forward).
		effAbort := o.ConfigFetchHedgeAbort
		if effAbort <= 0 {
			effAbort = DefaultConfigFetchHedgeAbort
		}
		if o.InitTimeout > 0 && o.InitTimeout <= effAbort {
			o.Logger.Warn("quonfig: InitTimeout <= config-fetch hedge abort; init-path heal-forward may be clipped",
				slog.Duration("init_timeout", o.InitTimeout),
				slog.Duration("hedge_abort", effAbort),
			)
		}
	}

	client := &Client{
		opts:               o,
		transport:          transport,
		initializationDone: make(chan struct{}),
		closeCh:            make(chan struct{}),
		resolvedFromIndex:  -1,
		sseStreamIndex:     -1,
	}

	if o.TelemetryEnabled() {
		client.telemetry = newTelemetrySubmitter(o)
		client.telemetry.Start()
	}

	// Delivery (SDK-key) mode: the active environment is determined by the SDK
	// key on the server, so a WithEnvironment / QUONFIG_ENVIRONMENT pin is
	// ignored. Warn once at init rather than silently dropping it. Datadir mode
	// honors the pin, so it is excluded.
	if o.DataDir == "" && o.Environment != "" {
		o.Logger.Warn(fmt.Sprintf("quonfig: environment %q was set but the client is in delivery (SDK-key) mode; the active environment is determined by the SDK key, so this setting is ignored (it applies only when loading from a local data dir)", o.Environment))
	}

	if o.DataDir != "" {
		envelope, err := loadWorkspaceEnvelope(o.DataDir, o.Environment)
		if err != nil {
			if client.telemetry != nil {
				client.telemetry.Stop()
			}
			return nil, err
		}
		client.installEnvelope(envelope, -1)
		client.finishInitialization(true)
		if o.DataDirAutoReload {
			client.startDatadirWatcher()
		}
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

// Close stops the supervised background workers, shuts down the SSE stream,
// and flushes pending telemetry.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		// Read shared workers under the lock so we synchronize with
		// startBackgroundWorkers, which runs asynchronously from the init
		// goroutine and may not yet have installed the SSE client or
		// supervisor. If they run after Close, they observe closeCh closed
		// and skip installation.
		c.mu.Lock()
		sse := c.sse
		sup := c.sup
		watcher := c.watcher
		c.mu.Unlock()
		if sse != nil {
			sse.Stop()
		}
		if sup != nil {
			sup.Stop()
		}
		if watcher != nil {
			watcher.Close()
		}
		if c.telemetry != nil {
			c.telemetry.Stop()
		}
	})
}

// startDatadirWatcher spawns the filesystem watcher that reloads the
// in-memory envelope on disk changes. Registration failures are logged and
// the SDK continues without auto-reload — read-only filesystems and
// immutable containers should never cause a panic.
func (c *Client) startDatadirWatcher() {
	logger := c.opts.Logger
	w := newDatadirWatcher(datadirWatcherConfig{
		Datadir:  c.opts.DataDir,
		Debounce: c.opts.DataDirAutoReloadDebounce,
		Logger:   logger,
		OnChange: c.reloadDatadir,
		OnError: func(err error) {
			logger.Warn("quonfig: datadir auto-reload watcher error",
				slog.String("datadir", c.opts.DataDir),
				slog.Any("err", err),
			)
		},
	})
	if !w.Start() {
		logger.Warn("quonfig: datadir auto-reload disabled (watcher failed to start)",
			slog.String("datadir", c.opts.DataDir),
		)
		return
	}
	c.mu.Lock()
	// If Close ran while we were starting, undo the registration.
	select {
	case <-c.closeCh:
		c.mu.Unlock()
		w.Close()
		return
	default:
	}
	c.watcher = w
	c.mu.Unlock()
	logger.Info("quonfig: datadir auto-reload watching",
		slog.String("datadir", c.opts.DataDir),
		slog.Duration("debounce", c.effectiveDebounce()),
	)
}

// reloadDatadir is the parse-then-swap fire site invoked by the watcher
// after a debounced burst. On parse failure we keep the old envelope and log
// rather than expose a broken state to readers.
func (c *Client) reloadDatadir() {
	envelope, err := loadWorkspaceEnvelope(c.opts.DataDir, c.opts.Environment)
	if err != nil {
		c.opts.Logger.Warn("quonfig: datadir auto-reload skipped (parse failed)",
			slog.String("datadir", c.opts.DataDir),
			slog.Any("err", err),
		)
		return
	}
	c.refreshMu.Lock()
	c.installEnvelope(envelope, -1)
	c.refreshMu.Unlock()
}

func (c *Client) effectiveDebounce() time.Duration {
	if c.opts.DataDirAutoReloadDebounce > 0 {
		return c.opts.DataDirAutoReloadDebounce
	}
	return DefaultDataDirAutoReloadDebounce
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

	c.logPollingMode()

	go func() {
		ctx := context.Background()
		if c.opts.InitTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.opts.InitTimeout)
			defer cancel()
		}

		_ = c.fetchAndInstall(ctx, true)

		c.startBackgroundWorkers()
	}()
}

// logPollingMode announces the chosen Layer 1 (SSE) and Layer 2 (fallback
// poll) configuration at startup. Per qfg-47c2.20 we emit one line so
// deployers can confirm the configured polling semantics.
func (c *Client) logPollingMode() {
	mode := "sse-only"
	if c.opts.FallbackPollEnabled {
		mode = "sse-with-fallback-poll"
	}
	if !c.opts.SSEEnabled {
		if c.opts.FallbackPollEnabled {
			mode = "fallback-poll-only"
		} else {
			mode = "no-background-refresh"
		}
	}
	attrs := []any{
		slog.String("mode", mode),
		slog.Bool("sse_enabled", c.opts.SSEEnabled),
		slog.Bool("fallback_poll_enabled", c.opts.FallbackPollEnabled),
	}
	if c.opts.FallbackPollEnabled {
		attrs = append(attrs, slog.Duration("fallback_poll_interval", c.opts.FallbackPollInterval))
		attrs = append(attrs, slog.Duration("fallback_poll_threshold", DefaultFallbackPollThreshold))
	}
	c.opts.Logger.Info("quonfig: polling configuration", attrs...)
}

// startBackgroundWorkers spawns SSE (Layer 1) and, when configured, the
// fallback poller (Layer 2) under a single supervisor that lives for the
// lifetime of the Client.
func (c *Client) startBackgroundWorkers() {
	if c.transport == nil {
		return
	}

	// Build the supervisor first so the SSE OnStateChange callback can feed
	// state directly into it and the Layer 2 poller. The supervisor is
	// stored on the Client so Close() can stop it.
	sup := newSupervisor(supervisorConfig{Logger: c.opts.Logger})

	var fp *fallbackPoller
	if c.opts.FallbackPollEnabled && c.opts.FallbackPollInterval > 0 {
		fp = newFallbackPoller(fallbackPollerConfig{
			Interval:  c.opts.FallbackPollInterval,
			Threshold: DefaultFallbackPollThreshold,
			Logger:    c.opts.Logger,
			Fetch: func(ctx context.Context) error {
				return c.fetchAndInstall(ctx, false)
			},
			OnEngage: func() {
				sup.setConnectionState(ConnStateFallingBack)
				c.opts.Logger.Warn("quonfig: Layer 2 fallback poller engaged (SSE disconnected past threshold)",
					slog.Duration("interval", c.opts.FallbackPollInterval),
					slog.Duration("threshold", DefaultFallbackPollThreshold),
				)
			},
			OnDisengage: func() {
				// Restore "connected" — we only reach this branch on SSE
				// reconnect, which has already set Connected via the state
				// callback ordering below. Setting it again is harmless and
				// guards the rare case where the poller's tick fires before
				// the SSE callback drains.
				sup.setConnectionState(ConnStateConnected)
				c.opts.Logger.Info("quonfig: Layer 2 fallback poller disengaged (SSE recovered)")
			},
		})
	}

	c.mu.Lock()
	c.sup = sup
	c.fallback = fp
	c.mu.Unlock()

	if fp != nil {
		// Register the Layer 2 worker with the supervisor so a panic inside
		// Run gets caught and the poller restarts with exponential backoff.
		sup.workers = append(sup.workers, worker{
			Layer: "2",
			Run:   fp.Run,
		})
	}

	sup.Start()

	if c.opts.SSEEnabled {
		c.startSSE()
	}
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
	// Chain the SDK's internal connection bookkeeping (supervisor state,
	// Layer 2 poller engagement) with any caller-supplied
	// OnSSEStateChange callback. The internal bookkeeping is the source of
	// truth for ConnectionState() and FallbackPollerActive().
	userCB := c.opts.OnSSEStateChange
	onStateChange := func(connected bool) {
		if connected {
			// The SSE stream is pinned to the primary leg (streamURLFor(0)) and
			// deliberately never repoints to the secondary — failover is an
			// HTTP-only property. Record the leg so SSEFailedOverToSecondary can
			// assert the stream stayed on primary (chaos scenario f05).
			c.mu.Lock()
			c.sseStreamIndex = 0
			c.mu.Unlock()
		}
		c.handleSSEStateChange(connected)
		if userCB != nil {
			userCB(connected)
		}
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
			// Reject-older guard: an SSE initial snapshot or update only installs
			// if it advances the held generation (qfg-7h5d.1.5).
			c.refreshMu.Lock()
			if c.shouldInstall(env) {
				// SSE is pinned to the primary leg and does not change which HTTP
				// leg ResolvedFrom reports, so pass -1.
				c.installEnvelope(env, -1)
			}
			c.refreshMu.Unlock()
		},
		OnStateChange: onStateChange,
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

// handleSSEStateChange routes an SSE connection edge to the supervisor and,
// if configured, the Layer 2 fallback poller. The supervisor is the source
// of truth for ConnectionState(); the poller decides when to engage based
// on how long the stream has been down.
func (c *Client) handleSSEStateChange(connected bool) {
	c.mu.RLock()
	sup := c.sup
	fp := c.fallback
	c.mu.RUnlock()

	if fp != nil {
		fp.SetSSEConnected(connected)
	}
	if sup == nil {
		return
	}
	if connected {
		sup.setConnectionState(ConnStateConnected)
		return
	}
	// On disconnect: only mark "disconnected" if the poller isn't already
	// engaged. The poller's OnEngage callback owns the "falling_back" edge
	// and we don't want a flicker between the two.
	if fp != nil && fp.Active() {
		return
	}
	sup.setConnectionState(ConnStateDisconnected)
}

// ConnectionState reports the SDK's customer-visible transport state. Values
// match the cross-SDK spec in
// project/plans/sdk-hardening-and-verification.md: initializing, connected,
// disconnected, falling_back. Returns "initializing" when background workers
// are disabled (e.g. WithDataDir, no API key).
func (c *Client) ConnectionState() ConnectionState {
	c.mu.RLock()
	sup := c.sup
	c.mu.RUnlock()
	if sup == nil {
		return ConnStateInitializing
	}
	return sup.ConnectionState()
}

// FallbackPollerActive reports whether the Layer 2 fallback poller is
// currently engaged (i.e. SSE has been down past the engagement threshold
// and the SDK is polling instead). Returns false when fallback polling is
// disabled or has not engaged.
func (c *Client) FallbackPollerActive() bool {
	c.mu.RLock()
	fp := c.fallback
	c.mu.RUnlock()
	if fp == nil {
		return false
	}
	return fp.Active()
}

// LastSuccessfulRefresh returns the wall-clock time of the most recent
// successful config install (either path). Zero value before the first
// install or when background workers are disabled.
func (c *Client) LastSuccessfulRefresh() time.Time {
	c.mu.RLock()
	sup := c.sup
	c.mu.RUnlock()
	if sup == nil {
		return time.Time{}
	}
	return sup.LastSuccessfulRefresh()
}

// HeldGeneration returns the Meta.Generation of the config the client is
// currently holding (0 before the first install, or when the server predates
// the watermark). A higher generation is strictly newer; this is the value the
// canonical-ordering guard compares against on every install path.
func (c *Client) HeldGeneration() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.heldGeneration
}

// ResolvedFrom reports which configured upstream leg produced the config the
// client is currently holding: "primary" (the first API URL), "secondary" (any
// later URL reached via failover), or "" before the first successful HTTP
// install. Reflects the HTTP config-fetch path; SSE installs do not change it.
func (c *Client) ResolvedFrom() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	switch {
	case c.resolvedFromIndex < 0:
		return ""
	case c.resolvedFromIndex == 0:
		return "primary"
	default:
		return "secondary"
	}
}

// ConfigInstallCount returns the number of times an envelope has been installed
// over the client's lifetime (every install path: initial fetch, failover
// fetch, SSE snapshot/update, fallback poll). The canonical-ordering guard
// keeps this from advancing on a same-or-older payload.
func (c *Client) ConfigInstallCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.configInstalls
}

// SSEFailedOverToSecondary reports whether the live SSE stream ever repointed
// to a non-primary leg. It is always false by design — SSE is pinned to the
// primary stream and failover is an HTTP-only property — and exists so the
// chaos suite can assert that invariant (scenario f05) and catch a regression
// that silently repoints the stream.
func (c *Client) SSEFailedOverToSecondary() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sseStreamIndex > 0
}

// fetchAndInstall drives one hedged config-fetch cycle and installs whatever
// arrives through the reject-older guard. The hedge fires the primary first and,
// only if it is slow or errors, fires the secondary in parallel; results are
// installed as they arrive so watermark-max falls out (higher generation wins, a
// late older payload never regresses, a late newer payload heals forward). On
// the init path ready() latches on the first successful install while the loop
// keeps draining so a late-but-newer leg heals forward before the call returns.
func (c *Client) fetchAndInstall(ctx context.Context, initial bool) error {
	if c.transport == nil {
		return nil
	}

	// Concurrent hedge cycles (a manual Refresh racing the fallback poller, say)
	// are safe and are NOT coalesced: each leg uses its own per-URL ETag slot
	// under etagMu, every install is serialized through refreshMu + the
	// reject-older guard (so an equal-or-older payload is a no-op and the install
	// count can't double), and each leg is bounded by hedgeAbort. Coalescing here
	// would make a manual Refresh() silently no-op whenever a background fetch is
	// in flight, which violates the Refresh() contract. The chaos refresh loop is
	// synchronous and init runs before any background worker starts, so there is
	// no real-world pileup to guard against.
	results := c.transport.FetchConfigsHedged(ctx, c.transport.hedgeDelay, c.transport.hedgeAbort)

	installedOnce := false
	fired := 0
	failures := 0
	var lastErr error

	for lr := range results {
		fired++
		if lr.Err != nil {
			failures++
			lastErr = lr.Err
			continue
		}
		res := lr.Res
		if res.NotChanged {
			continue
		}
		// Reject-older guard + install are atomic under refreshMu against every
		// other install path (SSE, datadir). installEnvelope sets the held
		// generation, install count, and resolved-from leg together.
		c.refreshMu.Lock()
		installed := false
		if c.shouldInstall(res.Envelope) {
			c.installEnvelope(res.Envelope, res.SourceIndex)
			installed = true
		}
		c.refreshMu.Unlock()
		if installed && !installedOnce {
			installedOnce = true
			if initial {
				c.finishInitialization(true)
			}
		}
	}

	if installedOnce {
		return nil
	}

	// Nothing installed. If every fired leg failed, surface the failure (init
	// path applies OnInitFailure). Otherwise every leg was a 304 (established
	// client, no change) — a no-op success.
	if fired > 0 && failures == fired {
		if initial {
			storedErr := lastErr
			// Normalize our own init-timeout DeadlineExceeded so callers'
			// errors.Is(err, ErrInitializationTimeout) is deterministic.
			if c.opts.InitTimeout > 0 && errors.Is(lastErr, context.DeadlineExceeded) {
				storedErr = c.initializationTimeoutError("")
			}
			c.mu.Lock()
			c.initializationErr = storedErr
			c.mu.Unlock()
			c.finishInitialization(false)
		}
		return lastErr
	}

	if initial {
		c.mu.Lock()
		c.initialized = true
		c.initializationErr = nil
		c.mu.Unlock()
		c.finishInitialization(true)
	}
	return nil
}

// shouldInstall applies the canonical reject-older rule to an incoming snapshot
// arriving on a network install path (poll, failover/hedged fetch, SSE initial
// snapshot, SSE update, fallback poller). The rule is the whole story — there is
// no source ranking:
//
//   - A fresh client (nothing installed yet) always accepts the first snapshot,
//     even at generation 0. A stale secondary payload can therefore seed a fresh
//     client, but...
//   - ...an established client installs only if the incoming Meta.Generation is
//     strictly greater than the held generation. An older payload is dropped, so
//     a late failover to a stale secondary can never move the client backward; a
//     later, newer primary win heals forward.
//   - A same-generation snapshot is a no-op (not strictly greater), so an equal
//     second leg can't re-install or flap.
//   - An unversioned snapshot (generation absent or <= 0 — a server that
//     predates the watermark, or one whose rev-count failed) carries no ordering
//     information, so it can't be rejected as "older". It installs exactly as it
//     did before this guard existed; rejecting it would freeze an established
//     client on stale config until a positive generation reappeared.
//
// Callers must hold c.refreshMu so the decision and the install that follows are
// atomic with respect to every other install path. Datadir install/reload is a
// local source of truth (generation is always 0) and intentionally bypasses this
// guard by calling installEnvelope directly.
func (c *Client) shouldInstall(envelope *ConfigEnvelope) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.configInstalls == 0 {
		return true
	}
	if envelope.Meta.Generation <= 0 {
		return true
	}
	return envelope.Meta.Generation > c.heldGeneration
}

// installEnvelope swaps in a freshly-built store/evaluator/resolver for the
// given envelope. sourceIndex is the baseURLs index of the HTTP leg that
// produced it (so ResolvedFrom reflects the leg actually holding the config);
// pass -1 for installs with no leg (datadir, SSE) to leave resolvedFromIndex
// untouched. heldGeneration, configInstalls, and resolvedFromIndex are set in
// one c.mu critical section so a reader can never observe a new generation
// paired with a stale resolved-from index.
func (c *Client) installEnvelope(envelope *ConfigEnvelope, sourceIndex int) {
	store := newRuntimeStore()
	store.Update(envelope)
	evaluator := newRuntimeEvaluator(store)
	resolver := newRuntimeResolver(store, evaluator, c.opts.EnvLookup)

	c.mu.Lock()
	c.store = store
	c.evaluator = evaluator
	c.resolver = resolver
	c.envID = envelope.Meta.Environment
	c.heldGeneration = envelope.Meta.Generation
	c.configInstalls++
	if sourceIndex >= 0 {
		c.resolvedFromIndex = sourceIndex
	}
	c.initialized = true
	c.initializationErr = nil
	onConfigUpdate := c.opts.OnConfigUpdate
	sup := c.sup
	c.mu.Unlock()

	if sup != nil {
		sup.recordSuccessfulRefresh()
	}

	if onConfigUpdate != nil {
		c.invokeOnConfigUpdate(onConfigUpdate)
	}
}

// invokeOnConfigUpdate calls the user-supplied OnConfigUpdate callback under
// a defer/recover so a panic in customer code cannot tear down the goroutine
// that called installEnvelope (the SSE callback goroutine, the polling
// supervisor's goroutine, or the init goroutine in fetchAndInstall). On
// recovery we log at ERROR with the panic value and stack — chaos scenario
// 10 asserts client.sdkLog('error', /callback|onConfigUpdate/i) matches this
// line. The SSE-side invokeOnEnvelope guard is still in place as a
// belt-and-suspenders catch for non-OnConfigUpdate panics in OnEnvelope
// (qfg-47c2.30).
func (c *Client) invokeOnConfigUpdate(fn func()) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		c.opts.Logger.Error("quonfig: OnConfigUpdate callback panicked; SDK continuing",
			slog.Any("panic", r),
			slog.String("stack", string(debug.Stack())),
			slog.String("layer", "1"),
			slog.String("reason", "callback_panic"),
		)
	}()
	fn()
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
