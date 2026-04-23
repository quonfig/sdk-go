package quonfig

import (
	"fmt"
	"strings"
)

// QuonfigSDKLoggingContextName is the top-level context name used by the
// ShouldLogPath convenience to inject the logger path for per-logger rule
// evaluation. It is load-bearing for api-telemetry's example-context
// auto-capture, so do not rename without updating the matching constants in
// the other SDKs.
const QuonfigSDKLoggingContextName = "quonfig-sdk-logging"

// LogLevel is the type for log level names.
type LogLevel string

const (
	LogLevelTrace LogLevel = "TRACE"
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
	LogLevelFatal LogLevel = "FATAL"
)

// ShouldLog returns true if a message at desiredLevel should be logged for the
// given configKey. The caller must pass the full stored key (e.g.
// "log-level.my-app") — the SDK does not auto-prefix "log-level.".
// desiredLevel is case-insensitive (e.g. "debug", "INFO"). Returns true if no
// config is found (log everything by default).
func (c *Client) ShouldLog(configKey string, desiredLevel string, ctx *ContextSet) bool {
	configured, ok, err := c.GetStringValue(configKey, ctx)
	if err != nil || !ok {
		return true
	}
	return logLevelOrder(desiredLevel) >= logLevelOrder(configured)
}

// ShouldLog returns true if a message at desiredLevel should be logged for the
// given configKey, using the bound context.
func (cb *ContextBoundClient) ShouldLog(configKey string, desiredLevel string) bool {
	return cb.client.ShouldLog(configKey, desiredLevel, cb.ctx)
}

// ShouldLogPath returns true if a message at desiredLevel should be logged
// for the given loggerPath. It is a higher-level convenience on top of
// ShouldLog: it uses the Client's LoggerKey (set via WithLoggerKey or
// Options.LoggerKey) as the underlying config key, and injects loggerPath
// into ctx under contexts["quonfig-sdk-logging"] = { "key": loggerPath } so
// a single log-level config can drive per-logger overrides via the normal
// rule engine.
//
// loggerPath is passed through verbatim — the SDK does not normalize it,
// so "MyApp::Services::Auth" stays as "MyApp::Services::Auth". Callers may
// pass any identifier shape their host language prefers (dotted, colon,
// slash, etc.) and author matching rules in the config against that exact
// shape.
//
// Panics if the Client has no LoggerKey set. Use the existing
// ShouldLog(configKey, ...) primitive directly if you need a different
// error-handling policy or want to evaluate an ad-hoc config key.
func (c *Client) ShouldLogPath(loggerPath string, desiredLevel string, ctx *ContextSet) bool {
	if c.opts.LoggerKey == "" {
		panic(fmt.Sprintf("quonfig: ShouldLogPath requires Options.LoggerKey (or WithLoggerKey) to be set; pass loggerPath=%q with a configured LoggerKey, or call ShouldLog(configKey, ...) directly", loggerPath))
	}

	loggerCtx := NewContextSet().WithNamedContextValues(QuonfigSDKLoggingContextName, map[string]interface{}{
		"key": loggerPath,
	})
	mergedCtx := Merge(ctx, loggerCtx)
	return c.ShouldLog(c.opts.LoggerKey, desiredLevel, mergedCtx)
}

// ShouldLogPath returns true if a message at desiredLevel should be logged
// for the given loggerPath, using the bound context. See Client.ShouldLogPath
// for full semantics.
func (cb *ContextBoundClient) ShouldLogPath(loggerPath string, desiredLevel string) bool {
	return cb.client.ShouldLogPath(loggerPath, desiredLevel, cb.ctx)
}

func logLevelOrder(level string) int {
	switch strings.ToUpper(level) {
	case "TRACE":
		return 0
	case "DEBUG":
		return 1
	case "INFO":
		return 2
	case "WARN":
		return 3
	case "ERROR":
		return 4
	case "FATAL":
		return 5
	default:
		return -1
	}
}
