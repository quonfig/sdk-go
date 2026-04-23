package quonfig

import (
	"strings"
	"testing"
)

// buildTestAppDatadir writes a workspace with a single `log-level.test-app`
// config whose rules key off `quonfig-sdk-logging.key` (i.e. the context
// spelling that ShouldLogPath injects):
//
//   - key starts with "foo."   -> DEBUG
//   - key starts with "noisy." -> ERROR
//   - otherwise                -> INFO (default rule)
func buildTestAppDatadir(t *testing.T) string {
	t.Helper()
	return writeWorkspaceFiles(t, `{"environments":["Production"]}`, map[string]string{
		"log-levels/log-level.test-app.json": `{
			"id": "log-level.test-app-id",
			"key": "log-level.test-app",
			"type": "log_level",
			"valueType": "log_level",
			"sendToClientSdk": false,
			"default": {
				"rules": [
					{"criteria": [{"operator": "ALWAYS_TRUE"}], "value": {"type": "log_level", "value": "INFO"}}
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
									"operator": "PROP_STARTS_WITH_ONE_OF",
									"valueToMatch": {"type": "string_list", "value": ["foo."]}
								}
							],
							"value": {"type": "log_level", "value": "DEBUG"}
						},
						{
							"criteria": [
								{
									"propertyName": "quonfig-sdk-logging.key",
									"operator": "PROP_STARTS_WITH_ONE_OF",
									"valueToMatch": {"type": "string_list", "value": ["noisy."]}
								}
							],
							"value": {"type": "log_level", "value": "ERROR"}
						},
						{"criteria": [{"operator": "ALWAYS_TRUE"}], "value": {"type": "log_level", "value": "INFO"}}
					]
				}
			]
		}`,
	})
}

func TestShouldLogPathEvaluatesPerLoggerRules(t *testing.T) {
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

	// foo.bar -> DEBUG rule. DEBUG emits debug/info/warn/error/fatal, not trace.
	if !client.ShouldLogPath("foo.bar", "DEBUG", nil) {
		t.Error("foo.bar DEBUG: expected true (DEBUG rule allows DEBUG)")
	}
	if !client.ShouldLogPath("foo.bar", "INFO", nil) {
		t.Error("foo.bar INFO: expected true (DEBUG rule allows INFO)")
	}
	if client.ShouldLogPath("foo.bar", "TRACE", nil) {
		t.Error("foo.bar TRACE: expected false (DEBUG rule does not allow TRACE)")
	}

	// noisy.thing -> ERROR rule. ERROR does NOT emit info.
	if client.ShouldLogPath("noisy.thing", "INFO", nil) {
		t.Error("noisy.thing INFO: expected false (ERROR rule suppresses INFO)")
	}
	if !client.ShouldLogPath("noisy.thing", "ERROR", nil) {
		t.Error("noisy.thing ERROR: expected true (ERROR rule allows ERROR)")
	}

	// otherwise -> INFO default rule. INFO does NOT emit debug.
	if client.ShouldLogPath("other.thing", "DEBUG", nil) {
		t.Error("other.thing DEBUG: expected false (INFO rule suppresses DEBUG)")
	}
	if !client.ShouldLogPath("other.thing", "INFO", nil) {
		t.Error("other.thing INFO: expected true (INFO rule allows INFO)")
	}
}

func TestShouldLogPathPassesNativeIdentifiersUnnormalized(t *testing.T) {
	// We route a Ruby-style identifier "MyApp::Services::Auth" through a rule
	// that requires the EXACT unnormalized string. If the SDK were to snake-case
	// / dot-ify the path (`my_app.services.auth`), this rule would not match.
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

	// Unnormalized identifier must match -> DEBUG rule -> INFO emits.
	if !client.ShouldLogPath("MyApp::Services::Auth", "INFO", nil) {
		t.Error("MyApp::Services::Auth INFO: expected true (DEBUG rule allows INFO)")
	}

	// What a normalizing SDK might send instead — we EXPECT this NOT to match
	// the exact-value rule, so it falls to the WARN default, which does not
	// emit info. This is the proof that no normalization occurred.
	if client.ShouldLogPath("my_app.services.auth", "INFO", nil) {
		t.Error("my_app.services.auth INFO: expected false (WARN default suppresses INFO)")
	}
}

func TestShouldLogPathPanicsWithoutLoggerKey(t *testing.T) {
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
			t.Fatal("expected ShouldLogPath to panic when LoggerKey is unset")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic value, got %T: %v", r, r)
		}
		if !strings.Contains(msg, "LoggerKey") {
			t.Errorf("expected panic message to mention LoggerKey, got: %s", msg)
		}
	}()

	client.ShouldLogPath("foo.bar", "INFO", nil)
}

func TestShouldLogPrimitiveStillWorks(t *testing.T) {
	// Deliberately omit LoggerKey — {configKey} is the escape hatch and must
	// continue to work exactly as before.
	dir := writeWorkspaceFiles(t, `{"environments":["Production"]}`, map[string]string{
		"log-levels/log-level.raw.json": `{
			"id": "log-level.raw",
			"key": "log-level.raw",
			"type": "log_level",
			"valueType": "log_level",
			"sendToClientSdk": false,
			"default": {
				"rules": [
					{"criteria": [{"operator": "ALWAYS_TRUE"}], "value": {"type": "log_level", "value": "INFO"}}
				]
			},
			"environments": [
				{
					"id": "Production",
					"rules": [
						{"criteria": [{"operator": "ALWAYS_TRUE"}], "value": {"type": "log_level", "value": "INFO"}}
					]
				}
			]
		}`,
	})

	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	t.Cleanup(client.Close)

	// INFO emits INFO but not DEBUG.
	if !client.ShouldLog("log-level.raw", "INFO", nil) {
		t.Error("log-level.raw INFO: expected true")
	}
	if client.ShouldLog("log-level.raw", "DEBUG", nil) {
		t.Error("log-level.raw DEBUG: expected false")
	}
}

func TestContextBoundClientShouldLogPathMergesBoundContext(t *testing.T) {
	// Rule: only match when tenant.id=alpha AND logger path starts with "svc."
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

	bound := client.WithContext(NewContextSet().WithNamedContextValues("tenant", map[string]interface{}{
		"id": "alpha",
	}))

	// Bound tenant=alpha + svc.* path -> DEBUG rule.
	if !bound.ShouldLogPath("svc.users", "DEBUG") {
		t.Error("bound svc.users DEBUG: expected true (tenant=alpha + svc.* rule)")
	}
	// Non-matching path -> WARN default, does not emit INFO.
	if bound.ShouldLogPath("other.path", "INFO") {
		t.Error("bound other.path INFO: expected false (WARN default)")
	}

	// Without the bound tenant context -> WARN default, DEBUG not emitted.
	if client.ShouldLogPath("svc.users", "DEBUG", nil) {
		t.Error("unbound svc.users DEBUG: expected false (missing tenant=alpha)")
	}
}
