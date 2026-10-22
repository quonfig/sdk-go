package quonfig

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Supervisor contract Test 5 — Callback-throw isolation (ITD v2026.10.22.2,
// qfg-6vno.25 / qfg-6vno.26). A panic from a user callback is caught, counted
// in quonfig_sdk_callback_errors_total{callback}, logged at ERROR with the
// value and stack, and is NOT a worker restart. The next envelope is still
// delivered and the callback runs again.

// t5Server serves gen 1 over HTTP and, on each SSE connect, the envelopes
// from next(). The stream stays open until drop is closed (first connection
// only) or the request ends.
func t5Server(t *testing.T, drop <-chan struct{}) *httptest.Server {
	t.Helper()
	var conns atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v2/configs") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"configs":[],"meta":{"version":"gen-1","environment":"Production","generation":1}}`))
			return
		}
		n := conns.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		if n == 1 {
			e2 := makeEnvelope("gen-2", "t5.key", "two")
			e2.Meta.Generation = 2
			e3 := makeEnvelope("gen-3", "t5.key", "three")
			e3.Meta.Generation = 3
			writeSSEEnvelope(w, f, e2)
			time.Sleep(50 * time.Millisecond)
			writeSSEEnvelope(w, f, e3)
			select {
			case <-drop:
				return
			case <-r.Context().Done():
				return
			}
		}
		_, _ = w.Write([]byte(":keepalive\n\n"))
		f.Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv
}

func t5Restarts(c *Client) int64 {
	n := c.metrics.restartTotal("")
	c.mu.RLock()
	sup := c.sup
	c.mu.RUnlock()
	if sup != nil {
		n += sup.WorkerRestartTotal("1") + sup.WorkerRestartTotal("2")
	}
	return n
}

func TestSupervisor_CallbackThrowIsolation_OnConfigUpdate(t *testing.T) {
	srv := t5Server(t, nil)
	logs := &lockedBuf{}
	var calls atomic.Int32
	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(true),
		withTestStreamURLOverride(srv.URL+"/api/v2/sse/config"),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
		WithLogger(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))),
		WithOnConfigUpdate(func() {
			if calls.Add(1) == 1 {
				panic("t5 user callback boom")
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	waitFor(t, 5*time.Second, func() bool {
		v, _, _ := client.GetStringValue("t5.key", nil)
		return v == "three" && calls.Load() >= 2
	}, "the envelope after the throw was not installed, or the callback did not run again")

	if got := client.metrics.callbackErrorTotal("on_config_update"); got != 1 {
		t.Errorf("callback_errors_total{on_config_update} = %d, want 1", got)
	}
	if got := t5Restarts(client); got != 0 {
		t.Errorf("worker_restart_total = %d across a callback throw, want 0", got)
	}
	if s := client.ConnectionState(); s != ConnStateConnected {
		t.Errorf("ConnectionState = %s, want connected", s)
	}
	out := logs.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "t5 user callback boom") ||
		!strings.Contains(out, "goroutine ") || !strings.Contains(out, "OnConfigUpdate callback") {
		t.Errorf("want an ERROR log naming the callback with the panic value and stack, got:\n%s", out)
	}
}

func TestSupervisor_CallbackThrowIsolation_OnConnectionStateChange(t *testing.T) {
	drop := make(chan struct{})
	srv := t5Server(t, drop)
	logs := &lockedBuf{}
	var calls atomic.Int32
	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(true),
		withTestStreamURLOverride(srv.URL+"/api/v2/sse/config"),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
		WithLogger(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))),
		WithSSEStateCallback(func(bool) {
			if calls.Add(1) == 1 {
				panic("t5 state callback boom")
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	waitFor(t, 5*time.Second, func() bool {
		v, _, _ := client.GetStringValue("t5.key", nil)
		return v == "three" && calls.Load() >= 1
	}, "envelopes not installed after the state callback threw")
	waitFor(t, time.Second, func() bool { return client.metrics.callbackErrorTotal("on_connection_state_change") == 1 },
		"callback_errors_total{on_connection_state_change} never reached 1")
	if got := t5Restarts(client); got != 0 {
		t.Errorf("worker_restart_total = %d across a callback throw, want 0", got)
	}
	if s := client.ConnectionState(); s != ConnStateConnected {
		t.Errorf("ConnectionState = %s, want connected", s)
	}

	// The next trigger (the stream drops and reconnects) runs the callback
	// again, without another callback error. The drop itself IS a restart.
	close(drop)
	waitFor(t, 5*time.Second, func() bool { return calls.Load() >= 3 }, "state callback did not run again after the throw")
	if got := client.metrics.callbackErrorTotal("on_connection_state_change"); got != 1 {
		t.Errorf("callback_errors_total{on_connection_state_change} = %d after later edges, want 1", got)
	}
	out := logs.String()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "t5 state callback boom") ||
		!strings.Contains(out, "goroutine ") || !strings.Contains(out, "callback") {
		t.Errorf("want an ERROR log naming the callback with the panic value and stack, got:\n%s", out)
	}
}
