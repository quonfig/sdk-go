package quonfig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// qfg-9dxb.6 (audit HIGH 1): an SSE event larger than the scanner's line
// limit used to be treated as a healthy connection. The client reconnected
// every ~250-500ms (backoff reset each time), every reconnect emitted a
// connected edge that cancelled the fallback poller's engage timer, and the
// poller therefore never engaged. Config stayed frozen with no signal.
//
// Now an oversized event counts as a failed connection: normal exponential
// backoff applies, and reconnects that keep hitting the oversized event do
// not report "connected", so the fallback poller engages and keeps config
// fresh over HTTP.
func TestSSEOversizedEventEngagesFallbackPollerWithBackoff(t *testing.T) {
	var sseConns, configHits atomic.Int32
	huge := strings.Repeat("x", 5*1024*1024) // > 4 MiB scanner limit

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v2/configs") {
			configHits.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"configs":[],"meta":{"version":"gen-1","environment":"Production","generation":1}}`))
			return
		}
		sseConns.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("id: gen-2\ndata: {\"configs\":[],\"pad\":\"" + huge + "\"}\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done() // hold the stream open like the real server
	}))
	t.Cleanup(srv.Close)

	start := time.Now()
	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(true),
		withTestStreamURLOverride(srv.URL+"/api/v2/sse/config"),
		WithFallbackPoll(true, 20*time.Millisecond),
		withTestFallbackPollThreshold(300*time.Millisecond),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	waitFor(t, 3*time.Second, func() bool { return client.FallbackPollerActive() && configHits.Load() >= 3 },
		"fallback poller never engaged although every SSE event was oversized")

	// Backoff: sleeps are at least 250ms, 500ms, 1s, ... so within 2.5s of
	// start at most 4 connection attempts can happen. The old behavior
	// (backoff reset on every oversized event) allowed ~10.
	if el := time.Since(start); el < 2500*time.Millisecond {
		time.Sleep(2500*time.Millisecond - el)
	}
	if n := sseConns.Load(); n > 4 {
		t.Errorf("SSE connection attempts in 2.5s = %d, want <= 4 (backoff not applied)", n)
	}
	if !client.FallbackPollerActive() {
		t.Errorf("fallback poller disengaged; an oversized-event reconnect reported connected")
	}
}

// After an oversized event, the first event that does get through reports
// the stream as connected again, so the fallback poller disengages.
func TestSSEOversizedEventThenNormalEventReconnects(t *testing.T) {
	var sseConns atomic.Int32
	huge := strings.Repeat("x", 5*1024*1024)
	states := make(chan bool, 16)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := sseConns.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		if n == 1 {
			_, _ = w.Write([]byte("id: gen-2\ndata: {\"pad\":\"" + huge + "\"}\n\n"))
			f.Flush()
			<-r.Context().Done()
			return
		}
		writeSSEEnvelope(w, f, makeEnvelope("gen-3", "k", "v"))
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	got := make(chan *ConfigEnvelope, 4)
	c := newSSEClient(sseClientConfig{
		URL:           srv.URL,
		APIKey:        "test-key",
		InitialDelay:  10 * time.Millisecond,
		OnEnvelope:    func(e *ConfigEnvelope) { got <- e },
		OnStateChange: func(v bool) { states <- v },
	})
	c.Start()
	t.Cleanup(c.Stop)

	select {
	case e := <-got:
		if e.Meta.Version != "gen-3" {
			t.Fatalf("got envelope %q, want gen-3", e.Meta.Version)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no envelope delivered after the oversized event")
	}
	waitFor(t, time.Second, func() bool {
		c.connectedMu.Lock()
		defer c.connectedMu.Unlock()
		return c.connected
	}, "stream not reported connected after a normal event")
}
