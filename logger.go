package quonfig

import "strings"

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
