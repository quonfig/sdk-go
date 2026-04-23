package quonfig

import (
	"context"
	"fmt"
	"log/slog"
)

// slog ecosystem adapter.
//
// This file ports the Reforge slog integration to Quonfig and wires it to the
// ShouldLogPath path-based evaluation primitive in logger.go. It gives users
// drop-in dynamic log-level control with Go's standard log/slog package.
//
// Two types are exposed:
//
//   - QuonfigHandler wraps any slog.Handler and gates records through
//     Client.ShouldLogPath. Use this when you want Quonfig to decide, per
//     loggerPath, which records reach the underlying handler.
//
//   - QuonfigLeveler implements slog.Leveler and returns the dynamically
//     configured level for a given loggerPath. Use this when you want slog's
//     built-in handler (JSON/Text) to pre-filter based on Level before the
//     record is even materialized.
//
// Both require Client.opts.LoggerKey to be non-empty (set via WithLoggerKey),
// matching ShouldLogPath's existing contract; constructing either with an
// unconfigured Client panics at construction time so the failure is loud and
// local.

// quonfigCtxKey is the context.Context key used to optionally attach a
// *ContextSet for per-call evaluation. Opaque on purpose — callers use the
// ContextWithContextSet / ContextSetFromContext helpers.
type quonfigCtxKey struct{}

// ContextWithContextSet returns a derived context carrying the given
// *ContextSet. QuonfigHandler.Handle pulls this ContextSet out and passes it
// to Client.ShouldLogPath, on top of the Client's GlobalContext. nil is a
// valid value and clears any previously attached ContextSet.
func ContextWithContextSet(ctx context.Context, cs *ContextSet) context.Context {
	return context.WithValue(ctx, quonfigCtxKey{}, cs)
}

// ContextSetFromContext returns the *ContextSet previously attached with
// ContextWithContextSet, or nil if none is attached.
func ContextSetFromContext(ctx context.Context) *ContextSet {
	if ctx == nil {
		return nil
	}
	if cs, ok := ctx.Value(quonfigCtxKey{}).(*ContextSet); ok {
		return cs
	}
	return nil
}

// slogLevelToQuonfigString converts a slog.Level to the TRACE/DEBUG/INFO/
// WARN/ERROR/FATAL text representation that Quonfig log-level configs use.
// The mapping matches ReforgeHQ/sdk-go: slog's levels are int-based with gaps
// (LevelDebug=-4, LevelInfo=0, LevelWarn=4, LevelError=8), so we extend with
// TRACE at LevelDebug-4 and FATAL at LevelError+4.
func slogLevelToQuonfigString(level slog.Level) string {
	switch {
	case level <= slog.LevelDebug-4:
		return "TRACE"
	case level <= slog.LevelDebug:
		return "DEBUG"
	case level <= slog.LevelInfo:
		return "INFO"
	case level <= slog.LevelWarn:
		return "WARN"
	case level <= slog.LevelError:
		return "ERROR"
	default:
		return "FATAL"
	}
}

// quonfigStringToSlogLevel is the inverse of slogLevelToQuonfigString, used
// by QuonfigLeveler.Level() to materialize a slog.Level from the configured
// text value. Unknown strings fall back to INFO so a misconfigured level
// doesn't silently drop everything.
func quonfigStringToSlogLevel(level string) slog.Level {
	switch logLevelOrder(level) {
	case 0: // TRACE
		return slog.LevelDebug - 4
	case 1: // DEBUG
		return slog.LevelDebug
	case 2: // INFO
		return slog.LevelInfo
	case 3: // WARN
		return slog.LevelWarn
	case 4: // ERROR
		return slog.LevelError
	case 5: // FATAL
		return slog.LevelError + 4
	default:
		return slog.LevelInfo
	}
}

// QuonfigHandler is a slog.Handler that gates records through Quonfig's
// dynamic per-logger level configuration. It wraps another slog.Handler
// (supplied by the caller, e.g. slog.NewJSONHandler(os.Stdout, nil)) and
// suppresses records whose level is below the Quonfig-configured level for
// the handler's loggerPath.
//
// Enabled, WithAttrs, and WithGroup delegate to the inner handler; Handle
// consults Client.ShouldLogPath before forwarding.
type QuonfigHandler struct {
	client     *Client
	inner      slog.Handler
	loggerPath string
}

// NewQuonfigHandler returns a QuonfigHandler bound to the given client and
// loggerPath. The loggerPath is passed through verbatim to ShouldLogPath —
// no normalization is applied, so rules may match the caller's native
// identifier shape (dotted, colon-delimited, slashed, etc.).
//
// Panics if client.opts.LoggerKey is empty. Configure it with
// quonfig.WithLoggerKey("log-level.my-app") when constructing the Client.
func NewQuonfigHandler(client *Client, inner slog.Handler, loggerPath string) *QuonfigHandler {
	if client == nil {
		panic("quonfig: NewQuonfigHandler requires a non-nil *Client")
	}
	if inner == nil {
		panic("quonfig: NewQuonfigHandler requires a non-nil inner slog.Handler")
	}
	if client.opts.LoggerKey == "" {
		panic(fmt.Sprintf("quonfig: NewQuonfigHandler requires Options.LoggerKey (or WithLoggerKey) to be set; loggerPath=%q", loggerPath))
	}
	return &QuonfigHandler{
		client:     client,
		inner:      inner,
		loggerPath: loggerPath,
	}
}

// Enabled reports whether the handler should process a record at the given
// level. It consults Quonfig via ShouldLogPath; if Quonfig says no, slog will
// skip record construction entirely, which is the cheap pre-filter the slog
// API is designed around.
func (h *QuonfigHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.client.ShouldLogPath(h.loggerPath, slogLevelToQuonfigString(level), ContextSetFromContext(ctx))
}

// Handle forwards a record to the inner handler if Quonfig says this record's
// level should log. This is belt-and-suspenders with Enabled: a caller that
// builds records by hand or bypasses slog.Logger still gets gated.
func (h *QuonfigHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.client.ShouldLogPath(h.loggerPath, slogLevelToQuonfigString(r.Level), ContextSetFromContext(ctx)) {
		return nil
	}
	return h.inner.Handle(ctx, r)
}

// WithAttrs returns a new QuonfigHandler whose inner handler carries the
// given attributes.
func (h *QuonfigHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &QuonfigHandler{
		client:     h.client,
		inner:      h.inner.WithAttrs(attrs),
		loggerPath: h.loggerPath,
	}
}

// WithGroup returns a new QuonfigHandler whose inner handler is nested under
// the given group.
func (h *QuonfigHandler) WithGroup(name string) slog.Handler {
	return &QuonfigHandler{
		client:     h.client,
		inner:      h.inner.WithGroup(name),
		loggerPath: h.loggerPath,
	}
}

// QuonfigLeveler is a slog.Leveler backed by Quonfig's dynamic per-logger
// level configuration. Hand it to slog.HandlerOptions.Level to have slog's
// built-in handlers pre-filter based on the current Quonfig-configured level
// for loggerPath.
//
// Level() is called on every record, so every record sees fresh
// configuration — flipping a level in the Quonfig UI takes effect immediately
// after the client's next config refresh, with no logger rebuild needed.
type QuonfigLeveler struct {
	client     *Client
	loggerPath string
}

// NewQuonfigLeveler returns a QuonfigLeveler bound to the given client and
// loggerPath. Panics if client.opts.LoggerKey is empty.
//
// Note: QuonfigLeveler does NOT thread a context.Context through to the
// Quonfig evaluator — slog.HandlerOptions.Level is contextless. If your rules
// depend on per-request context (tenant, user, etc.), use QuonfigHandler
// instead and attach a ContextSet via ContextWithContextSet.
func NewQuonfigLeveler(client *Client, loggerPath string) *QuonfigLeveler {
	if client == nil {
		panic("quonfig: NewQuonfigLeveler requires a non-nil *Client")
	}
	if client.opts.LoggerKey == "" {
		panic(fmt.Sprintf("quonfig: NewQuonfigLeveler requires Options.LoggerKey (or WithLoggerKey) to be set; loggerPath=%q", loggerPath))
	}
	return &QuonfigLeveler{
		client:     client,
		loggerPath: loggerPath,
	}
}

// Level returns the current slog.Level for the bound loggerPath, derived by
// reading the Client's LoggerKey config with the logger-path context
// injected. Unknown/missing config falls back to INFO.
func (l *QuonfigLeveler) Level() slog.Level {
	// Resolve the configured text level for this loggerPath. We go through
	// GetStringValue + the same context-injection ShouldLogPath uses, rather
	// than calling ShouldLog / ShouldLogPath in a probe loop, so the returned
	// slog.Level reflects the configured level exactly (not a rounded-down
	// approximation from binary-searching the level ladder).
	loggerCtx := NewContextSet().WithNamedContextValues(QuonfigSDKLoggingContextName, map[string]interface{}{
		"key": l.loggerPath,
	})
	configured, ok, err := l.client.GetStringValue(l.client.opts.LoggerKey, loggerCtx)
	if err != nil || !ok {
		return slog.LevelInfo
	}
	return quonfigStringToSlogLevel(configured)
}
