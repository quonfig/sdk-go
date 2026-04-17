package quonfig

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewClientWithDataDirLoadsWorkspace(t *testing.T) {
	client, err := NewClient(
		WithDataDir(workspaceFixtureDir(t)),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	t.Cleanup(client.Close)

	value, ok, err := client.GetStringValue("brand.new.string", nil)
	if err != nil {
		t.Fatalf("GetStringValue returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected brand.new.string to resolve")
	}
	if value != "hello.world" {
		t.Fatalf("expected hello.world, got %q", value)
	}

	on, found := client.FeatureIsOn("always.true", nil)
	if !found || !on {
		t.Fatalf("expected always.true to be on, got found=%v on=%v", found, on)
	}
}

func TestNewClientWithDataDirAndExplicitEnvironment(t *testing.T) {
	dir := writeWorkspace(t,
		`{"prod":"Production","staging":"Staging"}`,
		"feature-flags/flag.json",
		`{
			"id":"cfg-1",
			"key":"flag",
			"type":"feature_flag",
			"valueType":"bool",
			"sendToClientSdk":false,
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":false}}]},
			"environments":[
				{"id":"Production","rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":true}}]},
				{"id":"Staging","rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":false}}]}
			]
		}`,
	)

	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	t.Cleanup(client.Close)

	on, found := client.FeatureIsOn("flag", nil)
	if !found || !on {
		t.Fatalf("expected Production flag to be on, got found=%v on=%v", found, on)
	}
}

func TestNewClientWithDataDirRequiresEnvironmentWhenWorkspaceIsAmbiguous(t *testing.T) {
	dir := writeWorkspace(t,
		`{"prod":"Production","staging":"Staging"}`,
		"feature-flags/flag.json",
		`{
			"id":"cfg-1",
			"key":"flag",
			"type":"feature_flag",
			"valueType":"bool",
			"sendToClientSdk":false,
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":false}}]},
			"environments":[
				{"id":"Production","rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":true}}]},
				{"id":"Staging","rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":false}}]}
			]
		}`,
	)

	_, err := NewClient(
		WithDataDir(dir),
		WithAllTelemetryDisabled(),
	)
	if err == nil {
		t.Fatal("expected ambiguous workspace to require explicit environment")
	}
	if !strings.Contains(err.Error(), "WithEnvironment") {
		t.Fatalf("expected WithEnvironment guidance, got %v", err)
	}
}

func TestWithDataDirRejectsEmptyPath(t *testing.T) {
	_, err := NewClient(WithDataDir(""))
	if err == nil {
		t.Fatal("expected WithDataDir to reject empty path")
	}
}

func TestNewClientWithDataDirSkipsInvalidWorkspaceFilesWhenOthersLoad(t *testing.T) {
	dir := writeWorkspaceFiles(t, `{"prod":"Production"}`, map[string]string{
		"feature-flags/good.json": `{
			"id":"cfg-1",
			"key":"flag",
			"type":"feature_flag",
			"valueType":"bool",
			"sendToClientSdk":false,
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":false}}]},
			"environments":[
				{"id":"Production","rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":true}}]}
			]
		}`,
		"feature-flags/bad.json": `{"id":`,
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

	on, found := client.FeatureIsOn("flag", nil)
	if !found || !on {
		t.Fatalf("expected valid workspace config to load despite invalid sibling file, got found=%v on=%v", found, on)
	}
}

func TestLoadWorkspaceEnvelopeForcesSendToClientSdkTrueForFeatureFlag(t *testing.T) {
	dir := writeWorkspaceFiles(t, `{"prod":"Production"}`, map[string]string{
		// feature_flag WITHOUT sendToClientSdk on disk
		"feature-flags/flag-a.json": `{
			"id":"flag-a",
			"key":"flag-a",
			"type":"feature_flag",
			"valueType":"bool",
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":true}}]},
			"environments":[{"id":"Production","rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":true}}]}]
		}`,
		// feature_flag WITH sendToClientSdk:false — must still be forced true
		"feature-flags/flag-b.json": `{
			"id":"flag-b",
			"key":"flag-b",
			"type":"feature_flag",
			"valueType":"bool",
			"sendToClientSdk":false,
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":true}}]},
			"environments":[{"id":"Production","rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":true}}]}]
		}`,
		// config with sendToClientSdk absent — stays false
		"configs/cfg-a.json": `{
			"id":"cfg-a",
			"key":"cfg-a",
			"type":"config",
			"valueType":"string",
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"string","value":"x"}}]}
		}`,
	})

	envelope, err := loadWorkspaceEnvelope(dir, "Production")
	if err != nil {
		t.Fatalf("loadWorkspaceEnvelope returned error: %v", err)
	}

	byKey := map[string]ConfigResponse{}
	for _, cfg := range envelope.Configs {
		byKey[cfg.Key] = cfg
	}

	if got := byKey["flag-a"].SendToClientSDK; got != true {
		t.Errorf("flag-a (field absent): want SendToClientSDK=true, got %v", got)
	}
	if got := byKey["flag-b"].SendToClientSDK; got != true {
		t.Errorf("flag-b (field false on disk): want SendToClientSDK=true, got %v", got)
	}
	if got := byKey["cfg-a"].SendToClientSDK; got != false {
		t.Errorf("cfg-a (config, field absent): want SendToClientSDK=false, got %v", got)
	}
}

func TestNewClientWithDataDirFailsWhenNoWorkspaceFilesLoad(t *testing.T) {
	dir := writeWorkspaceFiles(t, `{"prod":"Production"}`, map[string]string{
		"feature-flags/bad.json": `{"id":`,
	})

	_, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
	)
	if err == nil {
		t.Fatal("expected invalid workspace to fail when no configs load")
	}
	if !strings.Contains(err.Error(), "failed to load any workspace configs") {
		t.Fatalf("expected workspace load failure, got %v", err)
	}
}

func workspaceFixtureDir(t *testing.T) string {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "integration-test-data", "data", "integration-tests")
}

func writeWorkspace(t *testing.T, environmentsJSON, configPath, configJSON string) string {
	t.Helper()
	return writeWorkspaceFiles(t, environmentsJSON, map[string]string{configPath: configJSON})
}

func writeWorkspaceFiles(t *testing.T, environmentsJSON string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "environments.json"), []byte(environmentsJSON), 0o644); err != nil {
		t.Fatalf("write environments.json: %v", err)
	}

	for path, contents := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir config dir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
			t.Fatalf("write config file: %v", err)
		}
	}

	return dir
}
