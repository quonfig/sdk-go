package quonfig

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// newSlogTestClient wires a client against the same test-app log-level config
// used by logger_test.go:
//   - key starts with "foo."   -> DEBUG
//   - key starts with "noisy." -> ERROR
//   - otherwise                -> INFO
func newSlogTestClient(t *testing.T) *Client {
	t.Helper()
	dir := buildTestAppDatadir(t)
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithLoggerKey("log-level.test-app"),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestSlogLevelToQuonfigStringRoundTrip(t *testing.T) {
	tests := []struct {
		in   slog.Level
		want string
	}{
		{slog.LevelDebug - 4, "TRACE"},
		{slog.LevelDebug, "DEBUG"},
		{slog.LevelInfo, "INFO"},
		{slog.LevelWarn, "WARN"},
		{slog.LevelError, "ERROR"},
		{slog.LevelError + 4, "FATAL"},
	}
	for _, tc := range tests {
		if got := slogLevelToQuonfigString(tc.in); got != tc.want {
			t.Errorf("slogLevelToQuonfigString(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestQuonfigStringToSlogLevel(t *testing.T) {
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"TRACE", slog.LevelDebug - 4},
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
		{"FATAL", slog.LevelError + 4},
		{"unknown", slog.LevelInfo}, // fallback
	}
	for _, tc := range tests {
		if got := quonfigStringToSlogLevel(tc.in); got != tc.want {
			t.Errorf("quonfigStringToSlogLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestQuonfigHandlerEnabledMatchesShouldLogPath(t *testing.T) {
	client := newSlogTestClient(t)

	tests := []struct {
		name       string
		loggerPath string
		level      slog.Level
		want       bool
	}{
		{"foo.bar debug allowed (DEBUG rule)", "foo.bar", slog.LevelDebug, true},
		{"foo.bar info allowed (DEBUG rule)", "foo.bar", slog.LevelInfo, true},
		{"noisy.x debug blocked (ERROR rule)", "noisy.x", slog.LevelDebug, false},
		{"noisy.x info blocked (ERROR rule)", "noisy.x", slog.LevelInfo, false},
		{"noisy.x error allowed (ERROR rule)", "noisy.x", slog.LevelError, true},
		{"other.y debug blocked (INFO default)", "other.y", slog.LevelDebug, false},
		{"other.y info allowed (INFO default)", "other.y", slog.LevelInfo, true},
		{"other.y warn allowed (INFO default)", "other.y", slog.LevelWarn, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewQuonfigHandler(client, slog.NewJSONHandler(&bytes.Buffer{}, nil), tc.loggerPath)
			if got := h.Enabled(context.Background(), tc.level); got != tc.want {
				t.Errorf("Enabled(%v) for %q = %v, want %v", tc.level, tc.loggerPath, got, tc.want)
			}
		})
	}
}

func TestQuonfigHandlerActualLogging(t *testing.T) {
	client := newSlogTestClient(t)

	tests := []struct {
		name       string
		loggerPath string
		log        func(*slog.Logger)
		wantOutput bool
	}{
		{
			name:       "foo.bar emits DEBUG (DEBUG rule)",
			loggerPath: "foo.bar",
			log:        func(l *slog.Logger) { l.Debug("msg-debug-foo") },
			wantOutput: true,
		},
		{
			name:       "foo.bar emits INFO (DEBUG rule)",
			loggerPath: "foo.bar",
			log:        func(l *slog.Logger) { l.Info("msg-info-foo") },
			wantOutput: true,
		},
		{
			name:       "noisy.thing suppresses DEBUG (ERROR rule)",
			loggerPath: "noisy.thing",
			log:        func(l *slog.Logger) { l.Debug("msg-debug-noisy") },
			wantOutput: false,
		},
		{
			name:       "noisy.thing suppresses INFO (ERROR rule)",
			loggerPath: "noisy.thing",
			log:        func(l *slog.Logger) { l.Info("msg-info-noisy") },
			wantOutput: false,
		},
		{
			name:       "noisy.thing emits ERROR (ERROR rule)",
			loggerPath: "noisy.thing",
			log:        func(l *slog.Logger) { l.Error("msg-error-noisy") },
			wantOutput: true,
		},
		{
			name:       "other.stuff suppresses DEBUG (INFO default)",
			loggerPath: "other.stuff",
			log:        func(l *slog.Logger) { l.Debug("msg-debug-other") },
			wantOutput: false,
		},
		{
			name:       "other.stuff emits INFO (INFO default)",
			loggerPath: "other.stuff",
			log:        func(l *slog.Logger) { l.Info("msg-info-other") },
			wantOutput: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			handler := NewQuonfigHandler(client, slog.NewJSONHandler(&buf, nil), tc.loggerPath)
			logger := slog.New(handler)
			tc.log(logger)

			out := buf.String()
			if tc.wantOutput && len(out) == 0 {
				t.Errorf("expected log output for %s, got none", tc.name)
			}
			if !tc.wantOutput && len(out) != 0 {
				t.Errorf("expected no log output for %s, got %q", tc.name, out)
			}
		})
	}
}

// Handle is the path a caller hits if they construct records by hand or use
// slog.NewLogLogger etc. Verify it is gated independently of Enabled.
func TestQuonfigHandlerHandleGatesDirectly(t *testing.T) {
	client := newSlogTestClient(t)

	var buf bytes.Buffer
	h := NewQuonfigHandler(client, slog.NewJSONHandler(&buf, nil), "noisy.thing")

	// DEBUG record on an ERROR-only path -> must NOT forward.
	rec := slog.NewRecord(time.Time{}, slog.LevelDebug, "should-not-emit", 0)
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected Handle to drop DEBUG record on ERROR path, got %q", buf.String())
	}

	// ERROR record on an ERROR-only path -> must forward.
	rec2 := slog.NewRecord(time.Time{}, slog.LevelError, "should-emit", 0)
	if err := h.Handle(context.Background(), rec2); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "should-emit") {
		t.Errorf("expected ERROR record to emit, got %q", buf.String())
	}
}

func TestQuonfigHandlerWithAttrsAndWithGroup(t *testing.T) {
	client := newSlogTestClient(t)

	var buf bytes.Buffer
	base := slog.NewJSONHandler(&buf, nil)
	h := NewQuonfigHandler(client, base, "foo.bar")

	withAttrs := h.WithAttrs([]slog.Attr{slog.String("service", "svc-x")})
	withGroup := withAttrs.WithGroup("req")

	logger := slog.New(withGroup)
	logger.Info("hello", "method", "GET")

	out := buf.String()
	if !strings.Contains(out, "service") || !strings.Contains(out, "svc-x") {
		t.Errorf("expected WithAttrs to propagate attrs, got %q", out)
	}
	if !strings.Contains(out, "req") || !strings.Contains(out, "method") {
		t.Errorf("expected WithGroup to propagate group, got %q", out)
	}
}

func TestQuonfigHandlerNativeIdentifierPassthrough(t *testing.T) {
	// Mirrors TestShouldLogPathPassesNativeIdentifiersUnnormalized: ensure a
	// native-shaped logger path ("MyApp::Services::Auth") passes through the
	// slog handler unchanged, so an exact-match rule fires. A normalized form
	// must NOT match the same rule.
	dir := writeWorkspaceFiles(t, `{"environments":["Production"]}`, map[string]string{
		"log-levels/log-level.native-id.json": `{
			"id": "log-level.native-id",
			"key": "log-level.native-id",
			"type": "log_level",
			"valueType": "log_level",
			"sendToClientSdk": false,
			"default": {
				"rules": [
					{"criteria": [{"operator": "ALWAYS_TRUE"}], "value": {"type": "log_level", "value": "WARN"}}
				]
			},
			"environments": [
				{
					"id": "Production",
					"rules": [
						{
							"criteria": [
								{
									"propertyName": "quonfig-sdk-logging.key",
									"operator": "PROP_IS_ONE_OF",
									"valueToMatch": {"type": "string_list", "value": ["MyApp::Services::Auth"]}
								}
							],
							"value": {"type": "log_level", "value": "DEBUG"}
						},
						{"criteria": [{"operator": "ALWAYS_TRUE"}], "value": {"type": "log_level", "value": "WARN"}}
					]
				}
			]
		}`,
	})
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithLoggerKey("log-level.native-id"),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	t.Cleanup(client.Close)

	// Native form -> DEBUG rule -> INFO emits.
	var bufNative bytes.Buffer
	slog.New(NewQuonfigHandler(client, slog.NewJSONHandler(&bufNative, nil), "MyApp::Services::Auth")).Info("native-info")
	if !strings.Contains(bufNative.String(), "native-info") {
		t.Errorf("expected native-form INFO to emit under DEBUG rule, got %q", bufNative.String())
	}

	// Normalized form -> falls through to WARN default -> INFO suppressed.
	var bufNorm bytes.Buffer
	slog.New(NewQuonfigHandler(client, slog.NewJSONHandler(&bufNorm, nil), "my_app.services.auth")).Info("normalized-info")
	if bufNorm.Len() != 0 {
		t.Errorf("expected normalized form to NOT match DEBUG rule (falls to WARN default), got %q", bufNorm.String())
	}
}

func TestQuonfigHandlerPanicsWithoutLoggerKey(t *testing.T) {
	dir := buildTestAppDatadir(t)
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		// no WithLoggerKey
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	t.Cleanup(client.Close)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected NewQuonfigHandler to panic without LoggerKey")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "LoggerKey") {
			t.Errorf("expected panic to mention LoggerKey, got %q", msg)
		}
	}()

	_ = NewQuonfigHandler(client, slog.NewJSONHandler(&bytes.Buffer{}, nil), "foo.bar")
}

func TestQuonfigLevelerReturnsConfiguredLevel(t *testing.T) {
	client := newSlogTestClient(t)

	tests := []struct {
		loggerPath string
		want       slog.Level
	}{
		{"foo.bar", slog.LevelDebug},
		{"noisy.thing", slog.LevelError},
		{"other.stuff", slog.LevelInfo},
	}
	for _, tc := range tests {
		t.Run(tc.loggerPath, func(t *testing.T) {
			leveler := NewQuonfigLeveler(client, tc.loggerPath)
			if got := leveler.Level(); got != tc.want {
				t.Errorf("Level() for %q = %v, want %v", tc.loggerPath, got, tc.want)
			}
		})
	}
}

func TestQuonfigLevelerDrivesSlogHandlerOptions(t *testing.T) {
	client := newSlogTestClient(t)

	// Use leveler bound to an ERROR-level path; slog's built-in handler should
	// drop DEBUG/INFO/WARN and emit only ERROR.
	var buf bytes.Buffer
	leveler := NewQuonfigLeveler(client, "noisy.thing")
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: leveler})
	logger := slog.New(handler)

	logger.Debug("dbg")
	logger.Info("nfo")
	logger.Warn("wrn")
	logger.Error("err")

	out := buf.String()
	if strings.Contains(out, "dbg") {
		t.Errorf("leveler should have suppressed DEBUG, got %q", out)
	}
	if strings.Contains(out, "nfo") {
		t.Errorf("leveler should have suppressed INFO, got %q", out)
	}
	if strings.Contains(out, "wrn") {
		t.Errorf("leveler should have suppressed WARN, got %q", out)
	}
	if !strings.Contains(out, "err") {
		t.Errorf("leveler should have allowed ERROR, got %q", out)
	}
}

func TestQuonfigLevelerPanicsWithoutLoggerKey(t *testing.T) {
	dir := buildTestAppDatadir(t)
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		// no WithLoggerKey
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	t.Cleanup(client.Close)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected NewQuonfigLeveler to panic without LoggerKey")
		}
	}()

	_ = NewQuonfigLeveler(client, "foo.bar")
}

func TestContextWithContextSetBridgesTenantToEvaluator(t *testing.T) {
	// Rule matches only when tenant.id=alpha AND path starts with svc.* .
	dir := writeWorkspaceFiles(t, `{"environments":["Production"]}`, map[string]string{
		"log-levels/log-level.tenant-app.json": `{
			"id": "log-level.tenant-app",
			"key": "log-level.tenant-app",
			"type": "log_level",
			"valueType": "log_level",
			"sendToClientSdk": false,
			"default": {
				"rules": [
					{"criteria": [{"operator": "ALWAYS_TRUE"}], "value": {"type": "log_level", "value": "WARN"}}
				]
			},
			"environments": [
				{
					"id": "Production",
					"rules": [
						{
							"criteria": [
								{
									"propertyName": "tenant.id",
									"operator": "PROP_IS_ONE_OF",
									"valueToMatch": {"type": "string_list", "value": ["alpha"]}
								},
								{
									"propertyName": "quonfig-sdk-logging.key",
									"operator": "PROP_STARTS_WITH_ONE_OF",
									"valueToMatch": {"type": "string_list", "value": ["svc."]}
								}
							],
							"value": {"type": "log_level", "value": "DEBUG"}
						},
						{"criteria": [{"operator": "ALWAYS_TRUE"}], "value": {"type": "log_level", "value": "WARN"}}
					]
				}
			]
		}`,
	})
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithLoggerKey("log-level.tenant-app"),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	t.Cleanup(client.Close)

	var buf bytes.Buffer
	h := NewQuonfigHandler(client, slog.NewJSONHandler(&buf, nil), "svc.users")

	// Without tenant context -> WARN default -> DEBUG suppressed.
	if h.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected DEBUG to be suppressed without tenant context")
	}

	// With tenant.id=alpha attached -> DEBUG rule matches -> DEBUG allowed.
	cs := NewContextSet().WithNamedContextValues("tenant", map[string]interface{}{"id": "alpha"})
	ctx := ContextWithContextSet(context.Background(), cs)
	if !h.Enabled(ctx, slog.LevelDebug) {
		t.Error("expected DEBUG to be allowed with tenant=alpha attached")
	}

	// Round-trip the accessor too.
	if got := ContextSetFromContext(ctx); got != cs {
		t.Errorf("ContextSetFromContext returned %v, want %v", got, cs)
	}
	if got := ContextSetFromContext(context.Background()); got != nil {
		t.Errorf("ContextSetFromContext on bare ctx = %v, want nil", got)
	}
}
