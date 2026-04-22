package quonfig

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// writeSSEEnvelope marshals an envelope and writes it as one SSE frame
// (id: <version>\ndata: <json>\n\n). Matches api-delivery/internal/serve/sse.go.
func writeSSEEnvelope(w http.ResponseWriter, f http.Flusher, env ConfigEnvelope) {
	b, _ := json.Marshal(env)
	fmt.Fprintf(w, "id: %s\ndata: %s\n\n", env.Meta.Version, b)
	f.Flush()
}

func makeEnvelope(version, key, val string) ConfigEnvelope {
	return ConfigEnvelope{
		Configs: []ConfigResponse{{
			Key:       key,
			ValueType: ValueTypeString,
			Default: RuleSet{
				Rules: []Rule{{Value: Value{Type: ValueTypeString, Value: val}}},
			},
		}},
		Meta: Meta{Version: version, Environment: "Production"},
	}
}

// TestSSEClientReceivesEventsAndReconnects spins up an SSE server that emits
// two events, disconnects abruptly, then on reconnect emits a third. The
// client should invoke onEnvelope 3 times across the two connections.
func TestSSEClientReceivesEventsAndReconnects(t *testing.T) {
	var connAttempts atomic.Int32
	recv := make(chan *ConfigEnvelope, 10)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth + version headers on every connection.
		user, pass, ok := r.BasicAuth()
		if !ok || user != "1" || pass != "test-key" {
			t.Errorf("bad basic auth: user=%q pass=%q ok=%v", user, pass, ok)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("missing Accept: text/event-stream header, got %q", r.Header.Get("Accept"))
		}
		if !strings.HasPrefix(r.Header.Get("X-Quonfig-SDK-Version"), "go-") {
			t.Errorf("missing X-Quonfig-SDK-Version header")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("response writer does not support flushing")
		}
		w.WriteHeader(http.StatusOK)

		attempt := connAttempts.Add(1)

		switch attempt {
		case 1:
			// First connection: two events, then abrupt close.
			writeSSEEnvelope(w, flusher, makeEnvelope("v1", "flag.a", "one"))
			writeSSEEnvelope(w, flusher, makeEnvelope("v2", "flag.a", "two"))
			// Fall through — return ends the request.
		case 2:
			// Second connection (reconnect): one event, then close.
			writeSSEEnvelope(w, flusher, makeEnvelope("v3", "flag.a", "three"))
		default:
			// Hold open quietly so the test doesn't spin.
			<-r.Context().Done()
		}
	}))
	defer server.Close()

	var mu sync.Mutex
	var envelopes []*ConfigEnvelope
	onEnv := func(env *ConfigEnvelope) {
		mu.Lock()
		envelopes = append(envelopes, env)
		mu.Unlock()
		select {
		case recv <- env:
		default:
		}
	}

	stateCh := make(chan bool, 8)
	onState := func(connected bool) {
		select {
		case stateCh <- connected:
		default:
		}
	}

	c := newSSEClient(sseClientConfig{
		URL:           server.URL,
		APIKey:        "test-key",
		UserAgent:     "go-0.0.8",
		OnEnvelope:    onEnv,
		OnStateChange: onState,
		InitialDelay:  1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
	})
	c.Start()
	defer c.Stop()

	deadline := time.After(3 * time.Second)
	got := 0
	for got < 3 {
		select {
		case <-recv:
			got++
		case <-deadline:
			mu.Lock()
			n := len(envelopes)
			mu.Unlock()
			t.Fatalf("timed out waiting for 3 envelopes, got %d; connAttempts=%d", n, connAttempts.Load())
		}
	}

	mu.Lock()
	if got, want := len(envelopes), 3; got != want {
		mu.Unlock()
		t.Fatalf("expected %d envelopes, got %d", want, got)
	}
	if v := envelopes[0].Configs[0].Default.Rules[0].Value.Value.(string); v != "one" {
		t.Errorf("envelope[0] = %q, want one", v)
	}
	if v := envelopes[2].Configs[0].Default.Rules[0].Value.Value.(string); v != "three" {
		t.Errorf("envelope[2] = %q, want three", v)
	}
	mu.Unlock()

	if got := connAttempts.Load(); got < 2 {
		t.Errorf("expected at least 2 connection attempts (reconnect), got %d", got)
	}

	// Drain state channel — we should have seen at least one connected=true.
	sawConnected := false
	timeout := time.After(500 * time.Millisecond)
drain:
	for {
		select {
		case s := <-stateCh:
			if s {
				sawConnected = true
			}
		case <-timeout:
			break drain
		}
	}
	if !sawConnected {
		t.Error("expected at least one OnStateChange(true) call")
	}
}

// TestSSEClientIgnoresKeepaliveComments verifies that SSE ": keepalive" lines
// (comments — matching what api-delivery emits every 30s) don't trigger
// spurious envelope callbacks or errors.
func TestSSEClientIgnoresKeepaliveComments(t *testing.T) {
	var callbacks atomic.Int32
	done := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)

		fmt.Fprint(w, ": keepalive\n\n")
		flusher.Flush()
		fmt.Fprint(w, ": another keepalive\n\n")
		flusher.Flush()
		writeSSEEnvelope(w, flusher, makeEnvelope("v1", "flag.k", "real"))
		fmt.Fprint(w, ": keepalive\n\n")
		flusher.Flush()

		<-r.Context().Done()
	}))
	defer server.Close()

	c := newSSEClient(sseClientConfig{
		URL:    server.URL,
		APIKey: "test-key",
		OnEnvelope: func(env *ConfigEnvelope) {
			if callbacks.Add(1) == 1 {
				close(done)
			}
		},
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
	})
	c.Start()
	defer c.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for envelope after keepalive comments; callbacks=%d", callbacks.Load())
	}

	// Let any spurious callbacks have a chance to fire.
	time.Sleep(50 * time.Millisecond)
	if got := callbacks.Load(); got != 1 {
		t.Errorf("expected exactly 1 envelope callback, got %d", got)
	}
}

// TestSSEClientStopUnblocks verifies Stop() cleanly shuts down an in-flight
// connection.
func TestSSEClientStopUnblocks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		<-r.Context().Done()
	}))
	defer server.Close()

	c := newSSEClient(sseClientConfig{
		URL:          server.URL,
		APIKey:       "test-key",
		OnEnvelope:   func(*ConfigEnvelope) {},
		InitialDelay: 1 * time.Millisecond,
	})
	c.Start()

	// Give it a chance to establish the connection.
	time.Sleep(50 * time.Millisecond)

	stopDone := make(chan struct{})
	go func() {
		c.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within 2s")
	}
}

// TestNewClientWithSSEDisabled verifies WithSSE(false) disables the background
// stream goroutine entirely — no SSE connection is attempted.
func TestNewClientWithSSEDisabled(t *testing.T) {
	var sseAttempts atomic.Int32
	var httpCalls atomic.Int32

	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sseAttempts.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer sseServer.Close()

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "v1")
		json.NewEncoder(w).Encode(makeEnvelope("v1", "flag.x", "off"))
	}))
	defer httpServer.Close()

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{httpServer.URL}),
		WithSSE(false),
		WithAllTelemetryDisabled(),
		withTestStreamURLOverride(sseServer.URL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	// Wait for initialization.
	_, _, _ = client.GetStringValue("flag.x", nil)

	// Give any accidental SSE goroutine time to fire.
	time.Sleep(200 * time.Millisecond)

	if got := sseAttempts.Load(); got != 0 {
		t.Errorf("expected 0 SSE connection attempts with WithSSE(false), got %d", got)
	}
	if got := httpCalls.Load(); got == 0 {
		t.Errorf("expected at least 1 HTTP call, got %d", got)
	}
}

// TestNewClientWithSSEEnabledConnects verifies that SSE is default-on and the
// background goroutine actually dials the stream URL after init completes.
func TestNewClientWithSSEEnabledConnects(t *testing.T) {
	sseConnected := make(chan struct{}, 1)

	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sseConnected <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		writeSSEEnvelope(w, flusher, makeEnvelope("v2", "flag.x", "streamed"))
		<-r.Context().Done()
	}))
	defer sseServer.Close()

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "v1")
		json.NewEncoder(w).Encode(makeEnvelope("v1", "flag.x", "polled"))
	}))
	defer httpServer.Close()

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{httpServer.URL}),
		WithAllTelemetryDisabled(),
		withTestStreamURLOverride(sseServer.URL),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	// Wait for initialization to complete.
	_, _, _ = client.GetStringValue("flag.x", nil)

	select {
	case <-sseConnected:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE server was never dialed with default WithSSE(true)")
	}

	// The streamed envelope should eventually overwrite the polled one.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		v, ok, err := client.GetStringValue("flag.x", nil)
		if err == nil && ok && v == "streamed" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("streamed envelope never installed via SSE")
}
