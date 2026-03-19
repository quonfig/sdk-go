package quonfig

import (
	"errors"
	"net/http"
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
	APIURL          string
	GlobalContext   *ContextSet
	InitTimeout     time.Duration
	OnInitFailure   OnInitFailure
	EnvLookup       EnvLookupFunc
	RefreshInterval time.Duration
	HTTPClient      *http.Client

	// Telemetry options
	CollectEvaluationSummaries bool
	ContextTelemetryMode       ContextTelemetryMode
	TelemetrySyncInterval      time.Duration
	TelemetryURL               string
}

// TelemetryEnabled returns true if a TelemetryURL is configured and any
// telemetry collection is enabled.  Without an explicit TelemetryURL telemetry
// is disabled — the SDK must never send telemetry to the config delivery API.
func (o *Options) TelemetryEnabled() bool {
	if o.TelemetryURL == "" {
		return false
	}
	return o.CollectEvaluationSummaries || o.ContextTelemetryMode != ContextTelemetryNone
}

func defaultOptions() Options {
	return Options{
		APIURL:                     "https://api.quonfig.com",
		InitTimeout:                10 * time.Second,
		OnInitFailure:              ReturnError,
		CollectEvaluationSummaries: true,
		ContextTelemetryMode:       ContextTelemetryPeriodicExample,
		TelemetrySyncInterval:      60 * time.Second,
		// TelemetryURL intentionally left empty — telemetry is disabled unless a
		// dedicated telemetry service URL is explicitly provided via WithTelemetryURL.
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

// WithAPIURL sets the base URL for the Quonfig API.
func WithAPIURL(url string) Option {
	return func(o *Options) error {
		if url == "" {
			return errors.New("API URL must not be empty")
		}
		o.APIURL = url
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

// WithAllTelemetryDisabled disables all telemetry collection.
func WithAllTelemetryDisabled() Option {
	return func(o *Options) error {
		o.CollectEvaluationSummaries = false
		o.ContextTelemetryMode = ContextTelemetryNone
		return nil
	}
}
