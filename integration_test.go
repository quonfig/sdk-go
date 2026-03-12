package quonfig_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	quonfig "github.com/quonfig/sdk-go"
)

const (
	testBackendKey  = "test-backend-key"
	testFrontendKey = "test-frontend-key"
	testEnvironment = "Production"
)

// projectRoot returns the root of the quonfig monorepo.
func projectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..")
}

// integrationTestDataDir returns the absolute path to the integration-test-data fixtures.
func integrationTestDataDir() string {
	return filepath.Join(projectRoot(), "integration-test-data", "data", "integration-tests")
}

// fixtureSDKKeysPath returns the path to the fixture SDK keys file.
func fixtureSDKKeysPath() string {
	return filepath.Join(projectRoot(), "api-delivery", "testdata", "fixture-sdk-keys.json")
}

// getFreePort finds an available TCP port.
func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if os.IsPermission(err) || strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("skipping integration test: cannot bind local port in this environment: %v", err)
		}
		t.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// startTestServer builds and starts the api-delivery server as a subprocess
// with FIXTURE_DIR pointing at the integration-test-data directory.
// Returns the base URL (e.g. "http://127.0.0.1:12345").
// The subprocess is killed via t.Cleanup.
func startTestServer(t *testing.T) string {
	t.Helper()

	fixtureDir := integrationTestDataDir()
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Skip("integration-test-data not available; skipping integration test")
	}

	keysPath := fixtureSDKKeysPath()
	if _, err := os.Stat(keysPath); err != nil {
		t.Skipf("fixture SDK keys not available at %s; skipping integration test", keysPath)
	}

	// Build the api-delivery binary
	serverDir := filepath.Join(projectRoot(), "api-delivery")
	binary := filepath.Join(t.TempDir(), "api-delivery")
	build := exec.Command("go", "build", "-o", binary, "./cmd/server")
	build.Dir = serverDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build api-delivery: %v\n%s", err, out)
	}

	port := getFreePort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("FIXTURE_DIR=%s", fixtureDir),
		fmt.Sprintf("SDK_KEYS_FILE=%s", keysPath),
	)
	cmd.Stdout = os.Stderr // let server logs appear in test output
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start api-delivery: %v", err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	// Wait for the server to become healthy
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			// Give server a moment to finish startup
			time.Sleep(100 * time.Millisecond)
			return baseURL
		}
		time.Sleep(50 * time.Millisecond)
	}

	cmd.Process.Kill()
	t.Fatal("api-delivery server did not start within 10 seconds")
	return ""
}

// newIntegrationClient creates a public sdk-go client and lets it initialize itself.
func newIntegrationClient(t *testing.T, serverURL, apiKey string) *quonfig.Client {
	t.Helper()

	// Build client
	client, err := quonfig.NewClient(
		quonfig.WithAPIKey(apiKey),
		quonfig.WithAPIURL(serverURL),
	)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

// ---------- Test A: Basic config fetch and evaluate ----------

func TestIntegration_FetchAndEvaluateConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	baseURL := startTestServer(t)
	client := newIntegrationClient(t, baseURL, testBackendKey)

	// Verify we loaded a meaningful number of configs
	keys := client.Keys()
	if len(keys) < 50 {
		t.Fatalf("expected at least 50 configs, got %d", len(keys))
	}
	t.Logf("loaded %d configs into SDK store", len(keys))

	// Get a simple string config that has no context-dependent rules
	val, ok, err := client.GetStringValue("brand.new.string", nil)
	if err != nil {
		t.Fatalf("GetStringValue error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find brand.new.string")
	}
	if val != "hello.world" {
		t.Errorf("expected %q, got %q", "hello.world", val)
	}

	// Get an int config
	intVal, ok, err := client.GetIntValue("brand.new.int", nil)
	if err != nil {
		t.Fatalf("GetIntValue error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find brand.new.int")
	}
	if intVal != 123 {
		t.Errorf("expected 123, got %d", intVal)
	}
}

// ---------- Test B: Feature flag evaluation ----------

func TestIntegration_FeatureFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	baseURL := startTestServer(t)
	client := newIntegrationClient(t, baseURL, testBackendKey)

	// always.true should be true regardless of context
	on, found := client.FeatureIsOn("always.true", nil)
	if !found {
		t.Fatal("expected to find always.true flag")
	}
	if !on {
		t.Error("expected always.true to be on")
	}

	// feature-flag.in-segment.positive: true when user.key is in the "users" segment
	// The "users" segment matches: michael, lauren, jeffrey, jeff, james, test
	ctx := quonfig.NewContextSet().WithNamedContextValues("user", map[string]interface{}{
		"key": "jeffrey",
	})
	on, found = client.FeatureIsOn("feature-flag.in-segment.positive", ctx)
	if !found {
		t.Fatal("expected to find feature-flag.in-segment.positive")
	}
	if !on {
		t.Error("expected feature-flag.in-segment.positive to be on for user jeffrey")
	}

	// Same flag with a user NOT in the segment should be false
	ctxOutside := quonfig.NewContextSet().WithNamedContextValues("user", map[string]interface{}{
		"key": "unknown-user",
	})
	on, found = client.FeatureIsOn("feature-flag.in-segment.positive", ctxOutside)
	if !found {
		t.Fatal("expected to find feature-flag.in-segment.positive")
	}
	if on {
		t.Error("expected feature-flag.in-segment.positive to be off for unknown user")
	}
}

// ---------- Test C: ETag polling behavior ----------

func TestIntegration_ETagPolling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	baseURL := startTestServer(t)
	client := newIntegrationClient(t, baseURL, testBackendKey)
	configCount := len(client.Keys())
	if configCount == 0 {
		t.Fatal("expected at least one config on first fetch")
	}
	t.Logf("first fetch: %d configs", configCount)

	// A manual refresh against the same fixture-backed server should be a no-op via ETag.
	if err := client.Refresh(); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if len(client.Keys()) != configCount {
		t.Errorf("expected store to retain %d configs after 304 refresh, got %d", configCount, len(client.Keys()))
	}
}

// ---------- Test D: Context-based evaluation ----------

func TestIntegration_ContextEvaluation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	baseURL := startTestServer(t)
	client := newIntegrationClient(t, baseURL, testBackendKey)

	// my-test-key has environment-specific rules in Production:
	// 1. If namespace.key is "present" -> "namespace-value"
	// 2. Default -> "my-test-value"

	// Without context: should get the Production default "my-test-value"
	val, ok, err := client.GetStringValue("my-test-key", nil)
	if err != nil {
		t.Fatalf("GetStringValue error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find my-test-key")
	}
	if val != "my-test-value" {
		t.Errorf("expected %q without context, got %q", "my-test-value", val)
	}

	// With namespace.key = "present": should get "namespace-value"
	ctx := quonfig.NewContextSet().WithNamedContextValues("namespace", map[string]interface{}{
		"key": "present",
	})
	val, ok, err = client.GetStringValue("my-test-key", ctx)
	if err != nil {
		t.Fatalf("GetStringValue with context error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find my-test-key with context")
	}
	if val != "namespace-value" {
		t.Errorf("expected %q with context, got %q", "namespace-value", val)
	}

	// With namespace.key = "something-else": should still get "my-test-value" (doesn't match rule)
	ctxOther := quonfig.NewContextSet().WithNamedContextValues("namespace", map[string]interface{}{
		"key": "something-else",
	})
	val, ok, err = client.GetStringValue("my-test-key", ctxOther)
	if err != nil {
		t.Fatalf("GetStringValue with other context error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find my-test-key with other context")
	}
	if val != "my-test-value" {
		t.Errorf("expected %q with non-matching context, got %q", "my-test-value", val)
	}
}

// ---------- Test E: Frontend vs Backend key filtering ----------

func TestIntegration_FrontendFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	baseURL := startTestServer(t)
	backendClient := newIntegrationClient(t, baseURL, testBackendKey)
	frontendClient := newIntegrationClient(t, baseURL, testFrontendKey)
	backendCount := len(backendClient.Keys())
	frontendCount := len(frontendClient.Keys())

	t.Logf("backend sees %d configs, frontend sees %d configs", backendCount, frontendCount)

	if frontendCount >= backendCount {
		t.Errorf("expected frontend (%d) to see fewer configs than backend (%d)", frontendCount, backendCount)
	}
	if frontendCount == 0 {
		t.Error("expected frontend to see at least some configs")
	}

	// james.test.key has sendToClientSdk=true, Production value is "test4"
	val, ok, err := frontendClient.GetStringValue("james.test.key", nil)
	if err != nil {
		t.Fatalf("frontend GetStringValue error: %v", err)
	}
	if !ok {
		t.Fatal("expected to find james.test.key via frontend key")
	}
	if val != "test4" {
		t.Errorf("expected %q for james.test.key via frontend, got %q", "test4", val)
	}

	// A backend-only key should be visible to the backend client but absent from the frontend client.
	if _, ok, err := backendClient.GetStringValue("brand.new.secret", nil); err != nil || !ok {
		t.Fatalf("expected backend client to see brand.new.secret, ok=%v err=%v", ok, err)
	}
	if _, ok, err := frontendClient.GetStringValue("brand.new.secret", nil); err != quonfig.ErrNotFound || ok {
		t.Fatalf("expected frontend client to hide brand.new.secret, ok=%v err=%v", ok, err)
	}
}
