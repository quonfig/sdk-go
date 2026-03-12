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
}

func defaultOptions() Options {
	return Options{
		APIURL:        "https://api.quonfig.com",
		InitTimeout:   10 * time.Second,
		OnInitFailure: ReturnError,
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
