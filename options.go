package quonfig

import (
	"errors"
	"log/slog"
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
	// The wire value is "shapes_only" — the value the SDK family agreed on
	// in qfg-6svs (see project/plans/sdk-1.0-unification.md, Section 1).
	ContextTelemetryShapes ContextTelemetryMode = "shapes_only"
	// ContextTelemetryPeriodicExample sends context shapes and periodic example values.
	ContextTelemetryPeriodicExample ContextTelemetryMode = "periodic_example"

	// contextTelemetryShapesLegacy is the pre-1.0 wire value for shape-only
	// context telemetry. The submitter still accepts it for one minor cycle
	// so callers passing the literal "shapes" keep working. Remove after
	// the unification deprecation window closes.
	contextTelemetryShapesLegacy = "shapes"
)

// Option is a functional option for configuring the Client.
type Option func(*Options) error

// EnvLookupFunc looks up an environment variable by name.
// Returns the value and whether it was found.
type EnvLookupFunc func(key string) (string, bool)

// DefaultDomain is the production domain used when QUONFIG_DOMAIN is unset
// and no explicit URL options are provided.
const DefaultDomain = "quonfig.com"

// Options holds all client configuration.
type Options struct {
	APIKey        string
	APIURLs       []string
	DataDir       string
	Environment   string
	GlobalContext *ContextSet
	InitTimeout   time.Duration
	OnInitFailure OnInitFailure

	// ConfigFetchTimeout bounds a single per-URL config-fetch attempt (the
	// initial fetch and every fallback-poller fetch alike). When a leg hangs —
	// accepts the connection but never responds — the attempt aborts after this
	// deadline so the next leg (e.g. the secondary) is reached inside the
	// overall InitTimeout instead of being starved until it. Zero means use
	// DefaultConfigFetchTimeout (~3s). This is a per-attempt deadline on the
	// HTTP config path only; it does not affect the long-lived SSE stream.
	ConfigFetchTimeout time.Duration
	EnvLookup          EnvLookupFunc
	HTTPClient         *http.Client

	// FallbackPollEnabled controls whether the Layer 2 fallback poller is
	// allowed to engage. The poller is idle while SSE is connected; it
	// engages only after SSE has been disconnected for FallbackPollThreshold
	// (default 120s) and ticks at FallbackPollInterval until SSE recovers.
	//
	// Default true: matches sdk-node/python/ruby/java so a customer who
	// turns SSE off or hits a network policy that blocks streaming still
	// gets a graceful poll-based refresh path instead of silent staleness.
	// Pass WithFallbackPoll(false, 0) to opt out.
	FallbackPollEnabled bool
	// FallbackPollInterval is how often the Layer 2 poller fetches once
	// engaged. Must be >0 when FallbackPollEnabled is true. Defaults to 60s.
	FallbackPollInterval time.Duration

	// Logger is the *slog.Logger used by the SDK to emit warnings (e.g. the
	// dev-context tokens loader). Defaults to slog.Default() when not set via
	// WithLogger. Internal helpers/goroutines that may emit warnings should
	// read from this field rather than calling slog.Default() directly so a
	// host app can route SDK output through its own handler.
	Logger *slog.Logger

	// apiURLsExplicit and telemetryURLExplicit track whether the caller set
	// the corresponding field via With* options. When true, applyDomainEnvOverride
	// leaves the field alone — explicit options always win over QUONFIG_DOMAIN.
	apiURLsExplicit      bool
	telemetryURLExplicit bool

	// OnConfigUpdate is called whenever the client installs a new config envelope
	// (i.e. after a successful fetch or data-dir load). It is called with the
	// client's internal mutex NOT held, so it is safe to call client methods
	// from within the callback.
	OnConfigUpdate func()

	// DataDirAutoReload enables filesystem watching when DataDir is set. The
	// SDK re-reads the datadir whenever a file inside changes, atomically
	// swaps the envelope on a successful parse, and fires OnConfigUpdate.
	// Default false — opt in for dev / git-pull workflows.
	DataDirAutoReload bool

	// DataDirAutoReloadDebounce coalesces filesystem-event bursts (atomic-rename
	// editor saves, git pull touching dozens of files). The SDK waits this long
	// after the most recent event before re-reading. Zero means use
	// DefaultDataDirAutoReloadDebounce (200ms).
	DataDirAutoReloadDebounce time.Duration

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

	// EnableQuonfigUserContext makes NewClient read ~/.quonfig/tokens.json
	// (written by `qfg login`) and merge { "quonfig-user": { "email":
	// <userEmail> } } into GlobalContext under any caller-supplied keys.
	//
	// Tri-state (nil = unset). Default ON, gated only by the presence of the
	// tokens file: production servers do not have it, so this is a no-op
	// there by construction. Precedence: this explicit pointer (if non-nil)
	// wins, else QUONFIG_DEV_CONTEXT ("true"/"false"), else true. Set it via
	// WithQuonfigUserContext(false) (or QUONFIG_DEV_CONTEXT=false) to opt out.
	EnableQuonfigUserContext *bool

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

	// testSSEReadTimeout, if non-zero, is forwarded to the SSE client as its
	// per-read idle deadline. The production default is 90s (3x the 30s
	// server heartbeat); tests that exercise stall detection use a much
	// smaller value so a scenario can fail fast rather than running for
	// minutes. Like testStreamURLOverride this is test-only — no public
	// With* accessor.
	testSSEReadTimeout time.Duration
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
		APIURLs:                    apiURLsForDomain(DefaultDomain),
		InitTimeout:                10 * time.Second,
		OnInitFailure:              ReturnError,
		ConfigFetchTimeout:         DefaultConfigFetchTimeout,
		SSEEnabled:                 true,
		FallbackPollEnabled:        true,
		FallbackPollInterval:       60 * time.Second,
		CollectEvaluationSummaries: true,
		ContextTelemetryMode:       ContextTelemetryPeriodicExample,
		TelemetrySyncInterval:      60 * time.Second,
		TelemetryURL:               telemetryURLForDomain(DefaultDomain),
	}
}

// apiURLsForDomain returns the ordered list of api base URLs derived from
// the given domain (e.g. "quonfig-staging.com" ->
// ["https://primary.quonfig-staging.com", "https://secondary.quonfig-staging.com"]).
func apiURLsForDomain(domain string) []string {
	return []string{
		"https://primary." + domain,
		"https://secondary." + domain,
	}
}

// telemetryURLForDomain returns the telemetry ingestion URL for the given
// domain (e.g. "quonfig-staging.com" -> "https://telemetry.quonfig-staging.com").
func telemetryURLForDomain(domain string) string {
	return "https://telemetry." + domain
}

// applyDomainEnvOverride checks the QUONFIG_DOMAIN environment variable and,
// if set to a non-empty value, derives APIURLs and TelemetryURL from it.
// Explicit With* options take precedence: if the caller set APIURLs via
// WithAPIURLs (apiURLsExplicit=true), the env var has no effect on APIURLs.
// Same for TelemetryURL. Resolution order is therefore:
//
//	explicit With* options > QUONFIG_DOMAIN env var > DefaultDomain
//
// This mirrors the CLI's QUONFIG_DOMAIN convention (cli/src/util/domain-urls.ts)
// and replaces the older per-URL env vars (QUONFIG_TELEMETRY_URL, etc.) which
// were removed in alpha — there is no backward-compat path.
func applyDomainEnvOverride(o *Options) {
	v, ok := os.LookupEnv("QUONFIG_DOMAIN")
	if !ok || v == "" {
		return
	}
	if !o.apiURLsExplicit {
		o.APIURLs = apiURLsForDomain(v)
	}
	if !o.telemetryURLExplicit {
		o.TelemetryURL = telemetryURLForDomain(v)
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

// resolveDevContextEnabled decides whether to inject quonfig-user.email.
// Precedence: an explicit WithQuonfigUserContext (non-nil pointer) wins,
// else QUONFIG_DEV_CONTEXT ("true"/"false"), else the default of true.
// The loader no-ops without a tokens file, so default-on is dead in prod.
func resolveDevContextEnabled(o *Options) bool {
	if o.EnableQuonfigUserContext != nil {
		return *o.EnableQuonfigUserContext
	}
	switch os.Getenv("QUONFIG_DEV_CONTEXT") {
	case "true":
		return true
	case "false":
		return false
	default:
		return true
	}
}

// applyAPIKeyEnvOverride checks the QUONFIG_BACKEND_SDK_KEY environment
// variable and, if set and no explicit WithSdkKey was provided, uses it
// as the API key. WithSdkKey takes precedence over the env var.
func applyAPIKeyEnvOverride(o *Options) {
	if o.APIKey != "" {
		return // explicit option takes precedence
	}
	if v, ok := os.LookupEnv("QUONFIG_BACKEND_SDK_KEY"); ok && v != "" {
		o.APIKey = v
	}
}

// WithSdkKey sets the SDK key for authentication.
func WithSdkKey(key string) Option {
	return func(o *Options) error {
		if key == "" {
			return errors.New("SDK key must not be empty")
		}
		o.APIKey = key
		return nil
	}
}

// WithAPIURLs sets an ordered list of base URLs for the Quonfig API.
// The client tries each URL in order, falling back to the next on failure.
// Setting this option is treated as an explicit override and takes precedence
// over the QUONFIG_DOMAIN env var.
func WithAPIURLs(urls []string) Option {
	return func(o *Options) error {
		if len(urls) == 0 {
			return errors.New("API URLs must not be empty")
		}
		o.APIURLs = urls
		o.apiURLsExplicit = true
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
// Default on (gated by the tokens file); this explicit option wins over
// the QUONFIG_DEV_CONTEXT env var, so pass false to opt out.
func WithQuonfigUserContext(enabled bool) Option {
	return func(o *Options) error {
		o.EnableQuonfigUserContext = &enabled
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

// WithConfigFetchTimeout sets the per-URL deadline for a single config-fetch
// attempt. It applies uniformly to the initial fetch and to every
// fallback-poller fetch. Each base URL in the failover list gets its own
// timeout, so a hung primary aborts after this duration and the secondary is
// tried within the remaining InitTimeout budget.
//
// Additive and backward-compatible: the default (DefaultConfigFetchTimeout,
// ~3s) already makes a hung upstream fail over, so existing callers need not
// set this. Pass a larger value only if a healthy upstream legitimately takes
// longer than 3s to answer a config fetch; pass a smaller value to fail over
// even faster. Must be positive.
//
// All-URLs-fail behavior is unchanged: if every leg times out (or otherwise
// errors), the initial fetch surfaces that failure through the configured
// OnInitFailure policy — ReturnError makes getters return
// ErrInitializationTimeout, ReturnZeroValue makes them return zero values.
func WithConfigFetchTimeout(d time.Duration) Option {
	return func(o *Options) error {
		if d <= 0 {
			return errors.New("config fetch timeout must be positive")
		}
		o.ConfigFetchTimeout = d
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

// WithFallbackPoll configures the Layer 2 fallback poller. The poller is
// idle while the SSE stream is connected; it engages only after SSE has
// been disconnected for the default 120s threshold, and ticks at the given
// interval until SSE recovers. Pass enabled=false to disable Layer 2
// entirely.
//
// Default: enabled with a 60s interval, matching sdk-node/python/ruby/java.
// Call WithFallbackPoll(false, 0) only if you want silent staleness during
// SSE outages.
func WithFallbackPoll(enabled bool, interval time.Duration) Option {
	return func(o *Options) error {
		if enabled && interval <= 0 {
			return errors.New("fallback poll interval must be positive when enabled")
		}
		if interval < 0 {
			return errors.New("fallback poll interval must not be negative")
		}
		o.FallbackPollEnabled = enabled
		o.FallbackPollInterval = interval
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

// WithLogger sets the *slog.Logger the SDK uses to emit warnings. When unset,
// the SDK falls back to slog.Default(). Symmetric with WithHTTPClient: pass
// the highest-level convenience type so callers who want a custom handler
// can do slog.New(myHandler) themselves.
//
// Note: this is distinct from WithLoggerKey, which configures the per-logger
// log-level config key consumed by ShouldLogPath.
func WithLogger(logger *slog.Logger) Option {
	return func(o *Options) error {
		if logger == nil {
			return errors.New("logger must not be nil")
		}
		o.Logger = logger
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

// WithContextTelemetryMode sets the context telemetry mode. The pre-1.0 wire
// value "shapes" is accepted as a deprecated alias for one minor cycle and
// normalized to the canonical ContextTelemetryShapes ("shapes_only").
func WithContextTelemetryMode(mode ContextTelemetryMode) Option {
	return func(o *Options) error {
		if mode == contextTelemetryShapesLegacy {
			mode = ContextTelemetryShapes
		}
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
// Setting this option is treated as an explicit override and takes
// precedence over the QUONFIG_DOMAIN env var.
func WithTelemetryURL(url string) Option {
	return func(o *Options) error {
		if url == "" {
			return errors.New("telemetry URL must not be empty")
		}
		o.TelemetryURL = url
		o.telemetryURLExplicit = true
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

// WithDataDirAutoReload enables filesystem watching for the configured
// DataDir. Default false — datadir mode is silent until you opt in.
//
// When enabled, the SDK walks the resolved datadir at startup, registers
// every subdirectory with fsnotify, debounces filesystem-event bursts
// (default DefaultDataDirAutoReloadDebounce, 200ms — tune with
// WithDataDirAutoReloadDebounce), then re-reads the workspace, parses it,
// and atomically swaps in the new envelope on success. The existing
// OnConfigUpdate callback fires on each successful swap; on parse failure
// the SDK keeps serving the previous envelope and the callback is NOT
// fired.
//
// Symlinks are resolved once at Start via filepath.EvalSymlinks — editing
// the file the symlink points at is detected, but atomic flips that
// retarget the symlink itself are not.
//
// Graceful degrade: if watch registration fails (read-only filesystem,
// immutable container, missing directory, EMFILE), the SDK logs at WARN
// via the configured WithLogger and continues serving the envelope it
// loaded at NewClient. It never panics on a watcher failure and
// NewClient does not return an error from a failed registration.
//
// Shutdown: Client.Close stops the watcher goroutine, releases the
// underlying fsnotify handle, and clears any pending debounce timer.
// There is no separate handle to manage — the watcher lifecycle is tied
// to the client.
func WithDataDirAutoReload(enabled bool) Option {
	return func(o *Options) error {
		o.DataDirAutoReload = enabled
		return nil
	}
}

// WithDataDirAutoReloadDebounce tunes how long the watcher waits after
// the most recent filesystem event before re-reading the datadir. Bursts
// of events (atomic-rename editor saves, `git pull` touching dozens of
// files) coalesce into a single re-read inside this window.
//
// Defaults to DefaultDataDirAutoReloadDebounce (200ms) — long enough to
// absorb the 3–5 events typical editors emit in <50ms, short enough that
// interactive edits feel immediate. Raise it if you have a noisy producer
// (continuously regenerating files) and would rather see one reload per
// second than per save. Lower it only if you have measured 200ms is
// meaningfully too slow.
//
// Has no effect unless WithDataDirAutoReload(true) is also set. A
// negative duration is rejected at option-apply time; zero falls back to
// the default.
func WithDataDirAutoReloadDebounce(d time.Duration) Option {
	return func(o *Options) error {
		if d < 0 {
			return errors.New("data dir auto reload debounce must not be negative")
		}
		o.DataDirAutoReloadDebounce = d
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
// plus the Layer 2 fallback poller, which is on by default (60s interval)
// and engages immediately when there is no SSE stream to be connected to.
// Override the interval with WithFallbackPoll(true, d), or disable polling
// entirely with WithFallbackPoll(false, 0).
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
