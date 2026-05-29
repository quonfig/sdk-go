package quonfig

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// qfg-pinh: In delivery (SDK-key) mode the active environment is determined by
// the SDK key, so a WithEnvironment / QUONFIG_ENVIRONMENT pin is ignored. We
// must emit a one-time WARN at init in that case so the setting isn't silently
// dropped. Datadir mode (where the pin is honored) must stay quiet, as must
// delivery mode with no pin.

const environmentIgnoredWarnFragment = "the active environment is determined by the SDK key"

func TestEnvironmentPinWarnsInDeliveryMode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithEnvironment("Production"),
		WithSSE(false),
		WithFallbackPoll(false, 0),
		WithLogger(logger),
		WithAllTelemetryDisabled(),
		WithInitTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	out := buf.String()
	if !strings.Contains(out, environmentIgnoredWarnFragment) {
		t.Fatalf("expected delivery-mode environment-ignored WARN, got: %q", out)
	}
	if !strings.Contains(out, "level=WARN") {
		t.Fatalf("expected WARN level for environment-ignored log, got: %q", out)
	}
	if !strings.Contains(out, "Production") {
		t.Fatalf("expected the ignored environment name in the log, got: %q", out)
	}
}

func TestEnvironmentPinFromEnvVarWarnsInDeliveryMode(t *testing.T) {
	t.Setenv("QUONFIG_ENVIRONMENT", "Staging")

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithSSE(false),
		WithFallbackPoll(false, 0),
		WithLogger(logger),
		WithAllTelemetryDisabled(),
		WithInitTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	out := buf.String()
	if !strings.Contains(out, environmentIgnoredWarnFragment) {
		t.Fatalf("expected delivery-mode environment-ignored WARN from env var, got: %q", out)
	}
	if !strings.Contains(out, "Staging") {
		t.Fatalf("expected the env-var environment name in the log, got: %q", out)
	}
}

func TestEnvironmentPinSilentInDatadirMode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	dir := emptyEnvelopeWorkspace(t)
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithLogger(logger),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	if out := buf.String(); strings.Contains(out, environmentIgnoredWarnFragment) {
		t.Fatalf("datadir mode honors the environment pin and must not warn, got: %q", out)
	}
}

func TestNoEnvironmentPinSilentInDeliveryMode(t *testing.T) {
	t.Setenv("QUONFIG_ENVIRONMENT", "")

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithSSE(false),
		WithFallbackPoll(false, 0),
		WithLogger(logger),
		WithAllTelemetryDisabled(),
		WithInitTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	if out := buf.String(); strings.Contains(out, environmentIgnoredWarnFragment) {
		t.Fatalf("delivery mode with no environment pin must not warn, got: %q", out)
	}
}
