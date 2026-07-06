package quonfig

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// qfg-41nh.26: The default (and every QUONFIG_DOMAIN-derived) API-URL list
// carries a primary and a secondary leg, and the SDK hedges/fails over between
// them. An explicit WithAPIURLs with a single entry silently drops the
// secondary, so we emit a one-time WARN at init pointing the caller at the fix.

const apiURLsFailoverWarnFragment = "explicit apiUrls disables automatic failover"

func TestSingleExplicitAPIURLWarnsFailoverLost(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{"https://primary.example.test"}),
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
	if !strings.Contains(out, apiURLsFailoverWarnFragment) {
		t.Fatalf("expected single-URL failover-lost WARN, got: %q", out)
	}
	if !strings.Contains(out, "level=WARN") {
		t.Fatalf("expected WARN level for failover-lost log, got: %q", out)
	}
}

func TestTwoExplicitAPIURLsDoNotWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{"https://primary.example.test", "https://secondary.example.test"}),
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

	if out := buf.String(); strings.Contains(out, apiURLsFailoverWarnFragment) {
		t.Fatalf("two explicit URLs keep failover and must not warn, got: %q", out)
	}
}

func TestDefaultAPIURLsDoNotWarnFailoverLost(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, err := NewClient(
		WithSdkKey("test-key"),
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

	if out := buf.String(); strings.Contains(out, apiURLsFailoverWarnFragment) {
		t.Fatalf("default URL list carries both legs and must not warn, got: %q", out)
	}
}
