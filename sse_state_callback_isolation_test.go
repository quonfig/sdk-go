package quonfig

import (
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newIsolationClient builds a client against shortLivedSSEServer: the stream
// connects once, drops at once, and every reconnect fails.
func newIsolationClient(t *testing.T, srv *httptest.Server, threshold time.Duration, cb func(bool)) *Client {
	t.Helper()
	client, err := NewClient(WithSdkKey("k"), WithAPIURLs([]string{srv.URL}),
		withTestStreamURLOverride(srv.URL+"/sse"),
		WithFallbackPoll(true, 20*time.Millisecond),
		withTestFallbackPollThreshold(threshold),
		WithSSEStateCallback(cb),
		WithAllTelemetryDisabled(), WithInitTimeout(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func pollUntil(d time.Duration, cond func() bool) bool {
	for deadline := time.Now().Add(d); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return true
		}
	}
	return cond()
}

// sseStopped reports whether the client's SSE reader goroutine has exited.
func sseStopped(c *Client) bool {
	c.mu.RLock()
	s := c.sse
	c.mu.RUnlock()
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// sseDispatching reports whether the client's user-callback dispatcher
// goroutine is running.
func sseDispatching(c *Client) bool {
	c.mu.RLock()
	s := c.sse
	c.mu.RUnlock()
	s.connectedMu.Lock()
	defer s.connectedMu.Unlock()
	return s.dispatching
}

// TestSSEBlockedUserCallbackDoesNotStallInternalState: a user state callback
// that never returns must not stop ConnectionState() from reporting the drop.
func TestSSEBlockedUserCallbackDoesNotStallInternalState(t *testing.T) {
	srv := shortLivedSSEServer()
	defer srv.Close()
	release := make(chan struct{})
	defer close(release)

	client := newIsolationClient(t, srv, time.Hour, func(bool) { <-release })
	defer client.Close()

	if !pollUntil(5*time.Second, func() bool { return client.ConnectionState() == ConnStateDisconnected }) {
		t.Fatalf("ConnectionState = %s, want disconnected while user callback is blocked", client.ConnectionState())
	}
}

// TestSSEBlockedUserCallbackDoesNotStallFallbackPoller: same, but the
// fallback poller must engage.
func TestSSEBlockedUserCallbackDoesNotStallFallbackPoller(t *testing.T) {
	srv := shortLivedSSEServer()
	defer srv.Close()
	release := make(chan struct{})
	defer close(release)

	client := newIsolationClient(t, srv, 50*time.Millisecond, func(bool) { <-release })
	defer client.Close()

	if !pollUntil(5*time.Second, client.FallbackPollerActive) {
		t.Fatalf("fallback poller not active (state %s) while user callback is blocked", client.ConnectionState())
	}
	if s := client.ConnectionState(); s == ConnStateConnected {
		t.Fatalf("ConnectionState = %s after the stream dropped", s)
	}
}

// TestSSESlowUserCallbackDoesNotDelayInternalState: a callback that takes 5s
// per edge must not hold back the internal disconnected state by that long.
func TestSSESlowUserCallbackDoesNotDelayInternalState(t *testing.T) {
	srv := shortLivedSSEServer()
	defer srv.Close()

	client := newIsolationClient(t, srv, time.Hour, func(bool) { time.Sleep(5 * time.Second) })
	defer client.Close()

	start := time.Now()
	if !pollUntil(2*time.Second, func() bool { return client.ConnectionState() == ConnStateDisconnected }) {
		t.Fatalf("ConnectionState = %s after %s, want disconnected (slow user callback delayed it)",
			client.ConnectionState(), time.Since(start))
	}
}

// flappingSSEServer accepts every SSE connection and ends it at once, so the
// client connects and drops repeatedly.
func flappingSSEServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
	}))
}

// TestSSEStateEdgesInOrderForBothSinks: with a flapping stream and a user
// callback of random speed, the internal sink and the user callback each see
// strictly alternating edges starting with connected.
func TestSSEStateEdgesInOrderForBothSinks(t *testing.T) {
	srv := flappingSSEServer()
	defer srv.Close()

	var mu sync.Mutex
	var internal, user []bool
	c := newSSEClient(sseClientConfig{
		URL: srv.URL, APIKey: "k", InitialDelay: time.Millisecond, MaxDelay: time.Millisecond,
		OnEnvelope: func(*ConfigEnvelope) {},
		OnStateChangeInternal: func(v bool) {
			mu.Lock()
			internal = append(internal, v)
			mu.Unlock()
		},
		OnStateChange: func(v bool) {
			time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond)
			mu.Lock()
			user = append(user, v)
			mu.Unlock()
		},
	})
	c.Start()
	pollUntil(5*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(user) >= 10
	})
	c.Stop()
	// Let the user dispatcher drain what Stop queued.
	pollUntil(5*time.Second, func() bool {
		c.connectedMu.Lock()
		defer c.connectedMu.Unlock()
		return !c.dispatching
	})

	mu.Lock()
	defer mu.Unlock()
	for name, edges := range map[string][]bool{"internal": internal, "user": user} {
		if len(edges) < 2 {
			t.Fatalf("%s: only %d edges", name, len(edges))
		}
		for i, v := range edges {
			if v != (i%2 == 0) {
				t.Fatalf("%s edges out of order at %d: %v", name, i, edges)
			}
		}
	}
	if len(internal) != len(user) {
		t.Fatalf("internal saw %d edges, user saw %d", len(internal), len(user))
	}
}

// TestCloseWithBlockedUserCallback: Close returns promptly while the user
// callback is blocked, and the only SDK goroutine left is the one running
// that callback. Once released it drains and exits too.
func TestCloseWithBlockedUserCallback(t *testing.T) {
	srv := shortLivedSSEServer()
	defer srv.Close()
	release := make(chan struct{})
	var entered atomic.Bool

	client := newIsolationClient(t, srv, time.Hour, func(bool) {
		entered.Store(true)
		<-release
	})
	if !pollUntil(5*time.Second, entered.Load) {
		t.Fatal("user callback never ran")
	}

	done := make(chan struct{})
	go func() { client.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		close(release)
		t.Fatal("Close did not return while the user callback was blocked")
	}

	if !sseStopped(client) {
		t.Fatal("SSE reader goroutine still running after Close returned")
	}
	// The one dispatcher is still inside the blocked user callback; the SDK
	// cannot unblock user code.
	if !sseDispatching(client) {
		t.Fatal("expected the dispatcher to still be inside the blocked callback")
	}

	close(release)
	if !pollUntil(5*time.Second, func() bool { return !sseDispatching(client) }) {
		t.Fatal("state dispatcher did not exit after the user callback returned")
	}
}

// TestUserCallbackCallsBackIntoClient: a callback that reads client state and
// calls Close from inside must not deadlock.
func TestUserCallbackCallsBackIntoClient(t *testing.T) {
	srv := shortLivedSSEServer()
	defer srv.Close()
	var cp atomic.Pointer[Client]
	closed := make(chan struct{})
	var once sync.Once

	client := newIsolationClient(t, srv, time.Hour, func(v bool) {
		c := cp.Load()
		for c == nil {
			time.Sleep(time.Millisecond)
			c = cp.Load()
		}
		_ = c.ConnectionState()
		_ = c.FallbackPollerActive()
		if !v {
			c.Close()
			once.Do(func() { close(closed) })
		}
	})
	cp.Store(client)

	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close called from the user callback did not return")
	}
	client.Close()
	if !pollUntil(5*time.Second, func() bool {
		return sseStopped(client) && !sseDispatching(client)
	}) {
		t.Fatal("SDK goroutines still running after Close from the callback")
	}
}
