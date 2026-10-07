package quonfig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// qfg-goi1.2.4: SSE connect hardening.

// Item 5: a panic in the user's OnSSEStateChange callback must not crash the
// process. It is logged at ERROR and later edges are still delivered.
func TestSSEClientRecoversFromOnStateChangePanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		writeSSEEnvelope(w, w.(http.Flusher), makeEnvelope("v1", "flag.a", "x"))
		// return: the stream drops, so the client sees connected then disconnected
	}))
	defer srv.Close()

	logger, logs := sseCaptureLogger()
	var mu sync.Mutex
	var edges []bool
	c := newSSEClient(sseClientConfig{
		URL:        srv.URL,
		APIKey:     "k",
		Logger:     logger,
		OnEnvelope: func(*ConfigEnvelope) {},
		OnStateChange: func(v bool) {
			mu.Lock()
			edges = append(edges, v)
			n := len(edges)
			mu.Unlock()
			if n == 1 {
				panic("simulated state callback panic")
			}
		},
		InitialDelay: time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	})
	c.Start()
	defer c.Stop()

	if !pollUntil(2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(edges) >= 3
	}) {
		mu.Lock()
		defer mu.Unlock()
		t.Fatalf("edges = %v, want later edges delivered after the panic", edges)
	}
	out := logs()
	if !strings.Contains(out, "level=ERROR") || !strings.Contains(out, "simulated state callback panic") {
		t.Fatalf("expected an ERROR log carrying the panic value; logs:\n%s", out)
	}
}
