package quonfig

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTmpHome points os.UserHomeDir() at a fresh temp dir for the duration of
// the subtest. On Unix and macOS, os.UserHomeDir() reads $HOME first, so
// t.Setenv is sufficient.
func withTmpHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func writeTokensFile(t *testing.T, home, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".quonfig"), 0o755); err != nil {
		t.Fatalf("mkdir .quonfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".quonfig", "tokens.json"), []byte(contents), 0o644); err != nil {
		t.Fatalf("write tokens.json: %v", err)
	}
}

func quonfigUserEmail(t *testing.T, cs *ContextSet) (string, bool) {
	t.Helper()
	if cs == nil {
		return "", false
	}
	v, ok := cs.GetContextValue("quonfig-user.email")
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func emptyEnvelopeWorkspace(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "environments.json"), []byte(`{"prod":"Production"}`), 0o644); err != nil {
		t.Fatalf("write environments.json: %v", err)
	}
	return dir
}

func TestLoadQuonfigUserContext_InjectsWhenFilePresent(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"bob@foo.com"}`)

	cs := loadQuonfigUserContext(nil, nil)
	got, ok := quonfigUserEmail(t, cs)
	if !ok {
		t.Fatalf("expected quonfig-user.email to be present, ContextSet=%+v", cs)
	}
	if got != "bob@foo.com" {
		t.Fatalf("expected bob@foo.com, got %q", got)
	}
}

func TestLoadQuonfigUserContext_NoOpWhenFileMissing(t *testing.T) {
	withTmpHome(t)

	cs := loadQuonfigUserContext(nil, nil)
	if cs != nil {
		t.Fatalf("expected nil ContextSet when file missing, got %+v", cs)
	}
}

func TestLoadQuonfigUserContext_NoOpWhenUnparseable(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{not valid json`)

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	cs := loadQuonfigUserContext(nil, nil)
	if cs != nil {
		t.Fatalf("expected nil ContextSet on unparseable file, got %+v", cs)
	}
	out := buf.String()
	if !strings.Contains(strings.ToLower(out), "dev-context") {
		t.Fatalf("expected slog warning mentioning dev-context, got: %s", out)
	}
}

func TestLoadQuonfigUserContext_NoOpWhenNoEmail(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"accessToken":"x"}`)

	cs := loadQuonfigUserContext(nil, nil)
	if cs != nil {
		t.Fatalf("expected nil ContextSet when no userEmail, got %+v", cs)
	}
}

func TestNewClientWithQuonfigUserContext_InjectsIntoGlobalContext(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"bob@foo.com"}`)
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	dir := emptyEnvelopeWorkspace(t)
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithQuonfigUserContext(true),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	got, ok := quonfigUserEmail(t, client.opts.GlobalContext)
	if !ok || got != "bob@foo.com" {
		t.Fatalf("expected GlobalContext.quonfig-user.email=bob@foo.com, got %q ok=%v", got, ok)
	}
}

func TestNewClientWithQuonfigUserContext_DisabledByDefault(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"bob@foo.com"}`)
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	dir := emptyEnvelopeWorkspace(t)
	customer := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"plan": "pro"})
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithGlobalContext(customer),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	if _, ok := quonfigUserEmail(t, client.opts.GlobalContext); ok {
		t.Fatalf("expected no quonfig-user injection when option disabled and no env var")
	}
	if v, ok := client.opts.GlobalContext.GetContextValue("user.plan"); !ok || v != "pro" {
		t.Fatalf("expected customer user.plan to remain, got %v ok=%v", v, ok)
	}
}

func TestNewClientWithQuonfigUserContext_FileMissingNoOp(t *testing.T) {
	withTmpHome(t)
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	dir := emptyEnvelopeWorkspace(t)
	customer := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"plan": "pro"})
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithQuonfigUserContext(true),
		WithGlobalContext(customer),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	if _, ok := quonfigUserEmail(t, client.opts.GlobalContext); ok {
		t.Fatalf("expected no quonfig-user injection when file missing")
	}
	if v, ok := client.opts.GlobalContext.GetContextValue("user.plan"); !ok || v != "pro" {
		t.Fatalf("expected customer user.plan to remain, got %v ok=%v", v, ok)
	}
}

func TestNewClientWithQuonfigUserContext_CustomerWinsOnCollision(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"bob@foo.com"}`)
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	dir := emptyEnvelopeWorkspace(t)
	customer := NewContextSet().WithNamedContextValues("quonfig-user", map[string]interface{}{"email": "override@x.com"})
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithQuonfigUserContext(true),
		WithGlobalContext(customer),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	got, ok := quonfigUserEmail(t, client.opts.GlobalContext)
	if !ok || got != "override@x.com" {
		t.Fatalf("expected customer-supplied override@x.com to win, got %q ok=%v", got, ok)
	}
}

func TestNewClientWithQuonfigUserContext_EnvVarEnables(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"bob@foo.com"}`)
	t.Setenv("QUONFIG_DEV_CONTEXT", "true")

	dir := emptyEnvelopeWorkspace(t)
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	got, ok := quonfigUserEmail(t, client.opts.GlobalContext)
	if !ok || got != "bob@foo.com" {
		t.Fatalf("expected env var to enable injection, got %q ok=%v", got, ok)
	}
}

func TestNewClientWithQuonfigUserContext_RuleEvalIntegration(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"bob@foo.com"}`)
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	dir := writeWorkspace(t,
		`{"prod":"Production"}`,
		"feature-flags/dev-only.json",
		`{
			"id":"cfg-dev-only",
			"key":"dev-only",
			"type":"feature_flag",
			"valueType":"bool",
			"sendToClientSdk":false,
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":false}}]},
			"environments":[
				{"id":"Production","rules":[
					{"criteria":[{"propertyName":"quonfig-user.email","operator":"PROP_IS_ONE_OF","valueToMatch":{"type":"string_list","value":["bob@foo.com"]}}],"value":{"type":"bool","value":true}},
					{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":false}}
				]}
			]
		}`,
	)

	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithQuonfigUserContext(true),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	on, found := client.FeatureIsOn("dev-only", nil)
	if !found {
		t.Fatalf("expected dev-only flag to be found")
	}
	if !on {
		t.Fatalf("expected dev-only flag to evaluate true via injected quonfig-user.email")
	}
}
