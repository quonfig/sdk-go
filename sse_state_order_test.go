package quonfig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// shortLivedSSEServer accepts one SSE connection that ends immediately after
// its 200 headers; every later connect gets a 503. Config fetches succeed.
func shortLivedSSEServer() *httptest.Server {
	var sse atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v2/configs") {
			_, _ = w.Write([]byte(`{"configs":[],"meta":{"version":"g1","environment":"Production","generation":1}}`))
			return
		}
		if sse.Add(1) == 1 {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			return
		}
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
}

// TestSSEClientDeliversStateEdgesInOrder: a stream that connects and drops
// at once must deliver connected then disconnected, in that order, even when
// the callback for the first edge is slow. Delivering each edge on its own
// goroutine let "disconnected" overtake "connected", leaving the client
// believing it was connected for the whole outage.
func TestSSEClientDeliversStateEdgesInOrder(t *testing.T) {
	srv := shortLivedSSEServer()
	defer srv.Close()

	var mu sync.Mutex
	var got []bool
	c := newSSEClient(sseClientConfig{
		URL: srv.URL, APIKey: "k", InitialDelay: time.Hour,
		OnEnvelope: func(*ConfigEnvelope) {},
		OnStateChange: func(v bool) {
			if v {
				time.Sleep(20 * time.Millisecond) // slow first edge
			}
			mu.Lock()
			got = append(got, v)
			mu.Unlock()
		},
	})
	c.Start()
	defer c.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	mu.Lock()
	edges := append([]bool(nil), got...)
	mu.Unlock()
	if len(edges) != 2 || !edges[0] || edges[1] {
		t.Fatalf("state edges = %v, want [true false]", edges)
	}

	// After Stop the dispatcher drains and exits; nothing is left running.
	c.Stop()
	for deadline := time.Now().Add(time.Second); ; time.Sleep(time.Millisecond) {
		c.connectedMu.Lock()
		running := c.dispatching
		c.connectedMu.Unlock()
		if !running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("state dispatcher still running after Stop")
		}
	}
}

// TestFallbackPollerEngagesAfterShortLivedStream is the client-level symptom:
// after a stream that connects and drops at once, with every reconnect
// failing, the fallback poller must engage and ConnectionState must not stay
// "connected".
func TestFallbackPollerEngagesAfterShortLivedStream(t *testing.T) {
	for i := 0; i < 5; i++ {
		srv := shortLivedSSEServer()
		client, err := NewClient(WithSdkKey("k"), WithAPIURLs([]string{srv.URL}),
			withTestStreamURLOverride(srv.URL+"/sse"),
			WithFallbackPoll(true, 20*time.Millisecond),
			withTestFallbackPollThreshold(50*time.Millisecond),
			WithAllTelemetryDisabled(), WithInitTimeout(5*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(2 * time.Second)
		for !client.FallbackPollerActive() && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		active, state := client.FallbackPollerActive(), client.ConnectionState()
		client.Close()
		srv.Close()
		if !active || state == ConnStateConnected {
			t.Fatalf("run %d: poller active=%v state=%s, want active and not connected", i, active, state)
		}
	}
}
