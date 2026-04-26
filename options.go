package quonfig

import (
	"errors"
	"net/http"
	"os"
	"time"
)

// OnInitFailure controls behavior when initialization times out.
type OnInitFailure int

const (
	// ReturnError causes getter methods to return an error if initialization times out.
	ReturnError OnInitFailure = iota
	// ReturnZeroValue causes getter methods to return zero values if initialization times out.
	ReturnZeroValue
)

// ContextTelemetryMode controls what context data the SDK sends to the telemetry backend.
type ContextTelemetryMode string

const (
	// ContextTelemetryNone disables context telemetry.
	ContextTelemetryNone ContextTelemetryMode = ""
	// ContextTelemetryShapes sends only context field names and types.
	ContextTelemetryShapes ContextTelemetryMode = "shapes"
	// ContextTelemetryPeriodicExample sends context shapes and periodic example values.
	ContextTelemetryPeriodicExample ContextTelemetryMode = "periodic_example"
)

// Option is a functional option for configuring the Client.
type Option func(*Options) error

// EnvLookupFunc looks up an environment variable by name.
// Returns the value and whether it was found.
type EnvLookupFunc func(key string) (string, bool)

// Options holds all client configuration.
type Options struct {
	APIKey          string
	APIURLs         []string
	DataDir         string
	Environment     string
	GlobalContext   *ContextSet
	InitTimeout     time.Duration
	OnInitFailure   OnInitFailure
	EnvLookup       EnvLookupFunc
	RefreshInterval time.Duration
	HTTPClient      *http.Client

	// OnConfigUpdate is called whenever the client installs a new config envelope
	// (i.e. after a successful fetch or data-dir load). It is called with the
	// client's internal mutex NOT held, so it is safe to call client methods
	// from within the callback.
	OnConfigUpdate func()

	// SSEEnabled controls whether a background SSE streamer is opened after
	// initialization. Default true. Set false for pure HTTP-poll behavior.
	// When DataDir is set (local dev) or no APIKey is configured, SSE is a
	// no-op regardless of this flag.
	SSEEnabled bool

	// OnSSEStateChange, if non-nil, is invoked whenever the background SSE
	// connection transitions between connected=true and connected=false.
	// Useful for emitting accurate "stream is up" metrics on the caller's
	// side (see load-gen's load_gen.sse_connected gauge). The callback may
	// run on any goroutine and should be cheap / non-blocking.
	OnSSEStateChange func(connected bool)

	// LoggerKey is the config key used by Client.ShouldLogPath to look up a
	// per-logger level rule (e.g. "log-level.my-app"). When set, callers can
	// use the higher-level ShouldLogPath(loggerPath, ...) convenience, which
	// injects loggerPath into the evaluation context as
	// contexts["quonfig-sdk-logging"] = { "key": loggerPath } so a single
	// log-level config can drive per-logger overrides.
	LoggerKey string

	// EnableQuonfigUserContext, when true, makes NewClient read
	// ~/.quonfig/tokens.json (written by `qfg login`) and merge
	// { "quonfig-user": { "email": <userEmail> } } into GlobalContext under
	// any caller-supplied keys. Default false. The env var
	// QUONFIG_DEV_CONTEXT=true also enables it. Production servers do not
	// have the tokens file, so this is a no-op there by construction.
	EnableQuonfigUserContext bool

	// Telemetry options
	CollectEvaluationSummaries bool
	ContextTelemetryMode       ContextTelemetryMode
	TelemetrySyncInterval      time.Duration
	TelemetryURL               string

	// testStreamURLOverride, if non-empty, is used verbatim for the SSE stream
	// connection instead of the URL derived from APIURLs. This is a test-only
	// escape hatch: no public With* accessor is exposed, so production callers
	// cannot set it. Tests that exercise the stream path against an
	// httptest.NewServer (which cannot provide a stream.* hostname) set this
	// field directly on Options after calling defaultOptions or after applying
	// their functional options.
	testStreamURLOverride string
}

// TelemetryEnabled returns true if a TelemetryURL is configured and any
// telemetry collection is enabled.
func (o *Options) TelemetryEnabled() bool {
	if o.TelemetryURL == "" {
		return false
	}
	return o.CollectEvaluationSummaries || o.ContextTelemetryMode != ContextTelemetryNone
}

func defaultOptions() Options {
	return Options{
		APIURLs: []string{
			"https://primary.quonfig.com",
		},
		InitTimeout:                10 * time.Second,
		OnInitFailure:              ReturnError,
		SSEEnabled:                 true,
		CollectEvaluationSummaries: true,
		ContextTelemetryMode:       ContextTelemetryPeriodicExample,
		TelemetrySyncInterval:      60 * time.Second,
		TelemetryURL:               "https://telemetry.quonfig.com",
	}
}

// applyTelemetryEnvOverride checks the QUONFIG_TELEMETRY_URL environment
// variable and, if set, overrides the TelemetryURL option. This is called
// after all functional options have been applied so the env var takes highest
// priority.
func applyTelemetryEnvOverride(o *Options) {
	if v, ok := os.LookupEnv("QUONFIG_TELEMETRY_URL"); ok {
		o.TelemetryURL = v
	}
}

// applyEnvironmentEnvOverride checks the QUONFIG_ENVIRONMENT environment
// variable and, if set and no explicit WithEnvironment was provided, uses it
// as the environment. WithEnvironment takes precedence over the env var.
func applyEnvironmentEnvOverride(o *Options) {
	if o.Environment != "" {
		return // explicit option takes precedence
	}
	if v, ok := os.LookupEnv("QUONFIG_ENVIRONMENT"); ok && v != "" {
		o.Environment = v
	}
}

// applyDevContextEnvOverride enables quonfig-user.email injection when
// QUONFIG_DEV_CONTEXT=true and no explicit WithQuonfigUserContext was set.
// An explicit option (true OR false) wins over the env var.
func applyDevContextEnvOverride(o *Options) {
	if o.EnableQuonfigUserContext {
		return
	}
	if v, ok := os.LookupEnv("QUONFIG_DEV_CONTEXT"); ok && v == "true" {
		o.EnableQuonfigUserContext = true
	}
}

// applyAPIKeyEnvOverride checks the QUONFIG_BACKEND_SDK_KEY environment
// variable and, if set and no explicit WithAPIKey was provided, uses it
// as the API key. WithAPIKey takes precedence over the env var.
func applyAPIKeyEnvOverride(o *Options) {
	if o.APIKey != "" {
		return // explicit option takes precedence
	}
	if v, ok := os.LookupEnv("QUONFIG_BACKEND_SDK_KEY"); ok && v != "" {
		o.APIKey = v
	}
}

// WithAPIKey sets the API key for authentication.
func WithAPIKey(key string) Option {
	return func(o *Options) error {
		if key == "" {
			return errors.New("API key must not be empty")
		}
		o.APIKey = key
		return nil
	}
}

// WithAPIURLs sets an ordered list of base URLs for the Quonfig API.
// The client tries each URL in order, falling back to the next on failure.
func WithAPIURLs(urls []string) Option {
	return func(o *Options) error {
		if len(urls) == 0 {
			return errors.New("API URLs must not be empty")
		}
		o.APIURLs = urls
		return nil
	}
}

// WithDataDir sets the local Quonfig workspace directory to load from disk.
func WithDataDir(path string) Option {
	return func(o *Options) error {
		if path == "" {
			return errors.New("data dir must not be empty")
		}
		o.DataDir = path
		return nil
	}
}

// WithEnvironment sets the environment ID/name used when loading from a local data dir.
func WithEnvironment(environment string) Option {
	return func(o *Options) error {
		if environment == "" {
			return errors.New("environment must not be empty")
		}
		o.Environment = environment
		return nil
	}
}

// WithGlobalContext sets the global context that is merged into every evaluation.
func WithGlobalContext(ctx *ContextSet) Option {
	return func(o *Options) error {
		o.GlobalContext = ctx
		return nil
	}
}

// WithQuonfigUserContext enables (or disables) injecting
// quonfig-user.email from ~/.quonfig/tokens.json into GlobalContext on
// NewClient. Customer-supplied GlobalContext keys win on collision.
// Default off; the env var QUONFIG_DEV_CONTEXT=true also enables it when
// no explicit option is set.
func WithQuonfigUserContext(enabled bool) Option {
	return func(o *Options) error {
		o.EnableQuonfigUserContext = enabled
		return nil
	}
}

// WithInitTimeout sets how long to wait for initial config loading before applying the OnInitFailure policy.
func WithInitTimeout(d time.Duration) Option {
	return func(o *Options) error {
		o.InitTimeout = d
		return nil
	}
}

// WithOnInitFailure sets the behavior when initialization times out.
func WithOnInitFailure(f OnInitFailure) Option {
	return func(o *Options) error {
		o.OnInitFailure = f
		return nil
	}
}

// WithEnvLookup sets a custom environment variable lookup function.
// By default, os.LookupEnv is used. This is useful for testing.
func WithEnvLookup(fn EnvLookupFunc) Option {
	return func(o *Options) error {
		o.EnvLookup = fn
		return nil
	}
}

// WithRefreshInterval enables background polling refreshes.
// A zero duration disables background refresh. Call Client.Refresh for manual polling.
func WithRefreshInterval(d time.Duration) Option {
	return func(o *Options) error {
		if d < 0 {
			return errors.New("refresh interval must not be negative")
		}
		o.RefreshInterval = d
		return nil
	}
}

// WithHTTPClient overrides the HTTP client used for config downloads.
func WithHTTPClient(client *http.Client) Option {
	return func(o *Options) error {
		if client == nil {
			return errors.New("HTTP client must not be nil")
		}
		o.HTTPClient = client
		return nil
	}
}

// WithCollectEvaluationSummaries enables or disables evaluation summary telemetry.
func WithCollectEvaluationSummaries(enabled bool) Option {
	return func(o *Options) error {
		o.CollectEvaluationSummaries = enabled
		return nil
	}
}

// WithContextTelemetryMode sets the context telemetry mode.
func WithContextTelemetryMode(mode ContextTelemetryMode) Option {
	return func(o *Options) error {
		o.ContextTelemetryMode = mode
		return nil
	}
}

// WithTelemetrySyncInterval sets how often telemetry is submitted to the backend.
func WithTelemetrySyncInterval(d time.Duration) Option {
	return func(o *Options) error {
		if d <= 0 {
			return errors.New("telemetry sync interval must be positive")
		}
		o.TelemetrySyncInterval = d
		return nil
	}
}

// WithTelemetryURL sets the telemetry ingestion endpoint.
func WithTelemetryURL(url string) Option {
	return func(o *Options) error {
		if url == "" {
			return errors.New("telemetry URL must not be empty")
		}
		o.TelemetryURL = url
		return nil
	}
}

// WithOnConfigUpdate sets a callback function that is called whenever the client
// receives and installs a new config envelope. This is useful for OpenFeature
// providers and other integrations that need to emit change events.
func WithOnConfigUpdate(fn func()) Option {
	return func(o *Options) error {
		o.OnConfigUpdate = fn
		return nil
	}
}

// WithLoggerKey sets the config key used by ShouldLogPath to look up a
// per-logger level rule (e.g. "log-level.my-app"). When set, callers can use
// ShouldLogPath(loggerPath, desiredLevel, ctx) and the SDK evaluates
// LoggerKey with contexts["quonfig-sdk-logging"] = { "key": loggerPath }
// merged into ctx. The existing ShouldLog(configKey, ...) primitive does not
// require this option.
func WithLoggerKey(key string) Option {
	return func(o *Options) error {
		if key == "" {
			return errors.New("logger key must not be empty")
		}
		o.LoggerKey = key
		return nil
	}
}

// WithAllTelemetryDisabled disables all telemetry collection.
func WithAllTelemetryDisabled() Option {
	return func(o *Options) error {
		o.CollectEvaluationSummaries = false
		o.ContextTelemetryMode = ContextTelemetryNone
		return nil
	}
}

// WithSSE enables or disables the background SSE streaming client.
// Default is true. When disabled, the SDK relies on the initial HTTP fetch
// plus any polling configured via WithRefreshInterval.
func WithSSE(enabled bool) Option {
	return func(o *Options) error {
		o.SSEEnabled = enabled
		return nil
	}
}

// WithSSEStateCallback registers a function that is invoked whenever the
// background SSE stream transitions between connected and disconnected.
// The callback receives true when a stream is live and false when it is not.
// Useful for emitting accurate connection-health metrics.
func WithSSEStateCallback(fn func(connected bool)) Option {
	return func(o *Options) error {
		o.OnSSEStateChange = fn
		return nil
	}
}

// withTestStreamURLOverride is a test-only option that forces the SSE client
// to dial the given URL verbatim instead of deriving it from APIURLs. It is
// unexported so production callers cannot set it. See
// Options.testStreamURLOverride for rationale.
func withTestStreamURLOverride(url string) Option {
	return func(o *Options) error {
		o.testStreamURLOverride = url
		return nil
	}
}
