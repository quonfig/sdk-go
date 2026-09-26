package quonfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// qfg-9dxb.2 (audit H1): the Layer 2 fallback poller must engage even when
// the SSE stream never connects (no connected->disconnected edge ever
// arrives), and must engage immediately when SSE is disabled.

// configCountingServer serves a valid envelope on /api/v2/configs and counts
// the hits. Any other path (e.g. the SSE stream) returns 503 so a stream
// pointed here never connects.
func configCountingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v2/configs") {
			hits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"configs":[],"meta":{"version":"gen-1","environment":"Production","generation":1}}`))
			return
		}
		http.Error(w, "stream unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestFallbackPollerEngagesWhenSSENeverConnects(t *testing.T) {
	srv, hits := configCountingServer(t)

	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(true),
		withTestStreamURLOverride(srv.URL+"/never-connects"),
		WithFallbackPoll(true, 20*time.Millisecond),
		withTestFallbackPollThreshold(50*time.Millisecond),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	waitFor(t, 3*time.Second, func() bool { return client.FallbackPollerActive() && hits.Load() >= 3 },
		"fallback poller never engaged although SSE never connected")
	if got := client.ConnectionState(); got != ConnStateFallingBack {
		t.Errorf("ConnectionState = %q, want %q", got, ConnStateFallingBack)
	}
}

func TestFallbackPollerEngagesImmediatelyWhenSSEDisabled(t *testing.T) {
	srv, hits := configCountingServer(t)

	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(false),
		WithFallbackPoll(true, 20*time.Millisecond),
		// Deliberately leave the threshold at the 120s default: with no SSE
		// stream there is nothing to wait for, so engagement must not wait.
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	waitFor(t, 3*time.Second, func() bool { return client.FallbackPollerActive() && hits.Load() >= 3 },
		"fallback poller never engaged with WithSSE(false)")
	if got := client.ConnectionState(); got != ConnStateFallingBack {
		t.Errorf("ConnectionState = %q, want %q", got, ConnStateFallingBack)
	}
}

// Unit level: a poller that never receives any state edge starts its engage
// timer at Run start.
func TestFallbackPollerEngagesWithoutAnyStateEdge(t *testing.T) {
	var fetches atomic.Int32
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  5 * time.Millisecond,
		Threshold: 10 * time.Millisecond,
		Fetch: func(ctx context.Context) error {
			fetches.Add(1)
			return nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = p.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	waitFor(t, time.Second, func() bool { return p.Active() && fetches.Load() >= 2 },
		"poller never engaged without a state edge")
}

// A supervisor restart of Run while SSE is connected must not arm the
// engage timer (no fresh edge will arrive to cancel it).
func TestFallbackPollerRestartWhileConnectedStaysIdle(t *testing.T) {
	var fetches atomic.Int32
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  5 * time.Millisecond,
		Threshold: 10 * time.Millisecond,
		Fetch: func(ctx context.Context) error {
			fetches.Add(1)
			return nil
		},
	})
	p.SetSSEConnected(true)
	<-p.stateCh // simulate the edge having been consumed by a prior Run

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = p.Run(ctx); close(done) }()
	time.Sleep(60 * time.Millisecond)
	cancel()
	<-done
	if p.Active() || fetches.Load() != 0 {
		t.Fatalf("restarted poller engaged while SSE connected: active=%v fetches=%d", p.Active(), fetches.Load())
	}
}
