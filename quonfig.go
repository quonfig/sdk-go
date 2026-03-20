// Package quonfig provides a client for fetching configuration and feature flags from the Quonfig API.
package quonfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
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

	// Env var override takes highest priority, after all options are applied.
	applyTelemetryEnvOverride(&o)

	client := &Client{
		opts:               o,
		initializationDone: make(chan struct{}),
		closeCh:            make(chan struct{}),
	}

	if o.TelemetryEnabled() {
		client.telemetry = newTelemetrySubmitter(o)
		client.telemetry.Start()
	}

	if o.APIKey == "" {
		client.initialized = true
		close(client.initializationDone)
		return client, nil
	}

	client.transport = newRuntimeTransport(o.APIURL, o.APIKey, o.HTTPClient)
	client.startInitialization()

	return client, nil
}

// SetStore sets the config store (used internally for wiring).
func (c *Client) SetStore(s configStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = s
}

// SetEvaluator sets the config evaluator.
func (c *Client) SetEvaluator(e ConfigEvaluator) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evaluator = e
}

// SetResolver sets the value resolver for ENV_VAR and decryption support.
func (c *Client) SetResolver(r ValueResolver) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resolver = r
}

// SetEnvironmentID sets the environment ID used for config evaluation.
func (c *Client) SetEnvironmentID(envID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.envID = envID
}

// Refresh performs a manual poll of GET /api/v2/configs using ETag caching.
func (c *Client) Refresh() error {
	if c.transport == nil {
		return nil
	}
	return c.fetchAndInstall(context.Background(), false)
}

// Close stops any background refresh loop and flushes pending telemetry.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
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
func (c *Client) GetJSONValue(key string, ctx *ContextSet) (interface{}, bool, error) {
	val, ok, err := c.resolve(key, ctx)
	if err != nil || !ok {
		return nil, false, err
	}
	s := val.StringValue()
	var result interface{}
	if jsonErr := json.Unmarshal([]byte(s), &result); jsonErr != nil {
		return nil, true, fmt.Errorf("parsing JSON: %w", jsonErr)
	}
	return result, true, nil
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

// resolve looks up a config and evaluates it against the given context.
func (c *Client) resolve(key string, ctx *ContextSet) (*Value, bool, error) {
	if err := c.awaitInitialization(key); err != nil {
		if c.opts.OnInitFailure == ReturnZeroValue && errors.Is(err, ErrInitializationTimeout) {
			return nil, false, nil
		}
		return nil, false, err
	}

	c.mu.RLock()
	store := c.store
	evaluator := c.evaluator
	resolver := c.resolver
	envID := c.envID
	globalContext := c.opts.GlobalContext
	telemetry := c.telemetry
	c.mu.RUnlock()

	if store == nil {
		return nil, false, ErrNotFound
	}
	cfg, ok := store.Get(key)
	if !ok {
		return nil, false, ErrNotFound
	}

	mergedCtx := Merge(globalContext, ctx)

	// Record context for telemetry (before evaluation, same as old sdk-go)
	if telemetry != nil {
		telemetry.RecordContext(mergedCtx)
	}

	// If we have an evaluator, use it for full rule evaluation with context
	if evaluator != nil {
		evalResult := evaluator.EvaluateConfigResponse(cfg, envID, mergedCtx)

		// Record evaluation for telemetry
		if telemetry != nil && evalResult != nil {
			telemetry.RecordEvaluation(evalResult)
		}

		if evalResult == nil || !evalResult.IsMatch || evalResult.Value == nil {
			return nil, false, nil
		}

		// Pass through the resolver if available (handles ENV_VAR, decryption)
		if resolver != nil {
			resolved, err := resolver.ResolveValue(evalResult.Value, cfg.Key, cfg.ValueType, envID, mergedCtx)
			if err != nil {
				return nil, false, err
			}
			return resolved, true, nil
		}
		return evalResult.Value, true, nil
	}

	// Fallback: return the first default rule's value (no evaluator available)
	if len(cfg.Default.Rules) > 0 {
		return &cfg.Default.Rules[0].Value, true, nil
	}
	return nil, false, nil
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

		if c.opts.RefreshInterval > 0 {
			c.startRefreshLoop()
		}
	}()
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
			c.mu.Lock()
			c.initializationErr = err
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

	store := newRuntimeStore()
	store.Update(result.Envelope)
	evaluator := newRuntimeEvaluator(store)
	resolver := newRuntimeResolver(store, evaluator, c.opts.EnvLookup)

	c.mu.Lock()
	c.store = store
	c.evaluator = evaluator
	c.resolver = resolver
	c.envID = result.Envelope.Meta.Environment
	c.initialized = true
	c.initializationErr = nil
	c.mu.Unlock()

	if initial {
		c.finishInitialization(true)
	}
	return nil
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
