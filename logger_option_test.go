package quonfig

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestWithLogger_RoutesDevContextWarn(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, "{not valid json")
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	dir := emptyEnvelopeWorkspace(t)
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithQuonfigUserContext(true),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	out := buf.String()
	if !strings.Contains(out, "could not parse tokens file") {
		t.Fatalf("expected custom logger to capture dev-context warn, got: %q", out)
	}
}

func TestWithLogger_DefaultsToSlogDefault(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, "{not valid json")
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

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

	if !strings.Contains(buf.String(), "could not parse tokens file") {
		t.Fatalf("expected slog.Default() to receive dev-context warn, got: %q", buf.String())
	}

	if client.opts.Logger == nil {
		t.Fatalf("expected NewClient to default opts.Logger to slog.Default(), got nil")
	}
}
