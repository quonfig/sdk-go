package quonfig

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A recovered OnConfigUpdate panic is a Layer 1 worker restart with reason
// callback_panic (chaos scenario 10), counted by the SDK itself (qfg-6vno.1).
func TestOnConfigUpdatePanicCountsCallbackPanicRestart(t *testing.T) {
	c := &Client{opts: Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	c.invokeOnConfigUpdate(func() { panic("boom") })
	if got := c.metrics.restartTotal("callback_panic"); got != 1 {
		t.Fatalf("restartTotal(callback_panic) = %d, want 1", got)
	}
}

// Every SSE attempt is a connect attempt, and every retry after a failed one
// is a connect_failed Layer 1 restart, into the metrics the caller passes.
func TestSSEClientCountsAttemptsAndRestarts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	m := &workerMetrics{}
	c := newSSEClient(sseClientConfig{
		URL:          srv.URL,
		APIKey:       "k",
		OnEnvelope:   func(*ConfigEnvelope) {},
		InitialDelay: time.Millisecond,
		MaxDelay:     2 * time.Millisecond,
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics:      m,
	})
	c.Start()
	deadline := time.Now().Add(3 * time.Second)
	for m.sseConnectAttempts.Load() < 4 {
		if time.Now().After(deadline) {
			t.Fatalf("connect attempts = %d, want >= 4", m.sseConnectAttempts.Load())
		}
		time.Sleep(2 * time.Millisecond)
	}
	c.Stop()
	attempts := m.sseConnectAttempts.Load()
	failed := m.restartTotal("connect_failed")
	if failed < attempts-1 || failed > attempts {
		t.Fatalf("connect_failed restarts = %d with %d attempts, want attempts-1..attempts", failed, attempts)
	}
	if got := m.restartTotal("stream_ended"); got != 0 {
		t.Fatalf("stream_ended restarts = %d, want 0 (no stream ever went live)", got)
	}
}
