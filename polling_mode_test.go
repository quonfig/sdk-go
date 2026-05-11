package quonfig

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// Tests for qfg-47c2.20: WithRefreshInterval deprecation shim, the startup
// polling-mode announcement, and the Client's exposed ConnectionState /
// FallbackPollerActive accessors when no background workers are running.

func TestPollingModeAnnouncedAtStartup(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithSSE(true),
		WithFallbackPoll(true, 30*time.Second),
		WithLogger(logger),
		WithAllTelemetryDisabled(),
		WithInitTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	// Give the init goroutine a few ms to run logPollingMode. We're not
	// waiting for the network fetch to complete.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), "polling configuration") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	out := buf.String()
	if !strings.Contains(out, "polling configuration") {
		t.Fatalf("expected polling-configuration startup log, got: %q", out)
	}
	if !strings.Contains(out, "mode=sse-with-fallback-poll") {
		t.Fatalf("expected mode=sse-with-fallback-poll in startup log, got: %q", out)
	}
	if !strings.Contains(out, "fallback_poll_interval=30s") {
		t.Fatalf("expected fallback_poll_interval=30s in startup log, got: %q", out)
	}
}

func TestWithRefreshIntervalEmitsDeprecationWarning(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithSSE(true),
		WithRefreshInterval(45*time.Second),
		WithLogger(logger),
		WithAllTelemetryDisabled(),
		WithInitTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), "WithRefreshInterval is deprecated") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	out := buf.String()
	if !strings.Contains(out, "WithRefreshInterval is deprecated") {
		t.Fatalf("expected deprecation warning for WithRefreshInterval, got: %q", out)
	}
	if !strings.Contains(out, "interval=45s") {
		t.Fatalf("expected interval=45s in deprecation warning, got: %q", out)
	}

	// And the shim must turn fallback polling ON with the same interval.
	if !client.opts.FallbackPollEnabled {
		t.Fatalf("expected FallbackPollEnabled=true after WithRefreshInterval, got false")
	}
	if got := client.opts.FallbackPollInterval; got != 45*time.Second {
		t.Errorf("FallbackPollInterval = %s, want 45s", got)
	}
}

func TestWithRefreshIntervalZeroDisablesFallbackPoll(t *testing.T) {
	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithRefreshInterval(0),
		WithAllTelemetryDisabled(),
		WithInitTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	if client.opts.FallbackPollEnabled {
		t.Errorf("expected FallbackPollEnabled=false when interval is 0, got true")
	}
}

func TestWithFallbackPollRejectsZeroIntervalWhenEnabled(t *testing.T) {
	_, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithFallbackPoll(true, 0),
	)
	if err == nil {
		t.Fatalf("expected error from WithFallbackPoll(true, 0), got nil")
	}
}

func TestConnectionStateInitializingWithoutTransport(t *testing.T) {
	// No API key → no transport → no background workers → state is
	// initializing per contract.
	client, err := NewClient(
		WithAPIURLs([]string{"https://example.test"}),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	if got := client.ConnectionState(); got != ConnStateInitializing {
		t.Errorf("ConnectionState() = %q, want %q", got, ConnStateInitializing)
	}
	if client.FallbackPollerActive() {
		t.Errorf("FallbackPollerActive() = true, want false (no workers)")
	}
	if !client.LastSuccessfulRefresh().IsZero() {
		t.Errorf("LastSuccessfulRefresh() = %s, want zero", client.LastSuccessfulRefresh())
	}
}

// End-to-end: simulate an SSE drop and verify that the Client correctly
// transitions through ConnectionState and engages the fallback poller after
// the configured threshold.
func TestHandleSSEStateChangeEngagesFallbackPoller(t *testing.T) {
	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithSSE(false), // skip the actual SSE dial; we drive state manually
		WithFallbackPoll(true, 5*time.Millisecond),
		WithAllTelemetryDisabled(),
		WithInitTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	// Wait for the supervisor to be wired (post-init goroutine).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		client.mu.RLock()
		ready := client.sup != nil && client.fallback != nil
		client.mu.RUnlock()
		if ready {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	client.mu.RLock()
	ready := client.sup != nil && client.fallback != nil
	client.mu.RUnlock()
	if !ready {
		t.Fatalf("background workers never wired up after init")
	}

	// Use a small threshold internally via SetSSEConnected — but the
	// Client wires fp with DefaultFallbackPollThreshold (120s) which is
	// too long for a unit test. Override the threshold by accessing the
	// fallback directly (test-only path).
	client.mu.Lock()
	client.fallback.cfg.Threshold = 10 * time.Millisecond
	client.mu.Unlock()

	client.handleSSEStateChange(true)
	if got := client.ConnectionState(); got != ConnStateConnected {
		t.Errorf("after connect ConnectionState = %q, want %q", got, ConnStateConnected)
	}

	client.handleSSEStateChange(false)
	// Wait for the poller to engage (immediate fetch on engage).
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if client.FallbackPollerActive() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !client.FallbackPollerActive() {
		t.Fatalf("FallbackPollerActive never became true after disconnect")
	}
	if got := client.ConnectionState(); got != ConnStateFallingBack {
		t.Errorf("after engagement ConnectionState = %q, want %q", got, ConnStateFallingBack)
	}

	client.handleSSEStateChange(true)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !client.FallbackPollerActive() {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if client.FallbackPollerActive() {
		t.Fatalf("FallbackPollerActive never became false after reconnect")
	}
	if got := client.ConnectionState(); got != ConnStateConnected {
		t.Errorf("after recover ConnectionState = %q, want %q", got, ConnStateConnected)
	}
}
