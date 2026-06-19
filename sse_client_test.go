package quonfig

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
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
	_, _ = fmt.Fprintf(w, "id: %s\ndata: %s\n\n", env.Meta.Version, b)
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

		_, _ = fmt.Fprint(w, ": keepalive\n\n")
		flusher.Flush()
		_, _ = fmt.Fprint(w, ": another keepalive\n\n")
		flusher.Flush()
		writeSSEEnvelope(w, flusher, makeEnvelope("v1", "flag.k", "real"))
		_, _ = fmt.Fprint(w, ": keepalive\n\n")
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
		_ = json.NewEncoder(w).Encode(makeEnvelope("v1", "flag.x", "off"))
	}))
	defer httpServer.Close()

	client, err := NewClient(
		WithSdkKey("test-key"),
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

// TestSSEClientReadTimeoutDropsStalledConnection verifies the SSE client
// drops a silently-stalled connection within the configured read deadline and
// reconnects. The "silent stall" failure mode (NAT timeout, LB half-close)
// previously left the SDK wedged forever — see qfg-47c2.10 / sse_client.go.
func TestSSEClientReadTimeoutDropsStalledConnection(t *testing.T) {
	var connAttempts atomic.Int32
	reconnected := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)

		attempt := connAttempts.Add(1)
		if attempt == 1 {
			// First conn: send one event, then stop writing — silent stall.
			writeSSEEnvelope(w, flusher, makeEnvelope("v1", "flag.s", "first"))
			<-r.Context().Done()
			return
		}
		// Subsequent conn: signal that the client reconnected.
		select {
		case reconnected <- struct{}{}:
		default:
		}
		writeSSEEnvelope(w, flusher, makeEnvelope("v2", "flag.s", "after-reconnect"))
		<-r.Context().Done()
	}))
	defer server.Close()

	c := newSSEClient(sseClientConfig{
		URL:          server.URL,
		APIKey:       "test-key",
		OnEnvelope:   func(*ConfigEnvelope) {},
		ReadTimeout:  100 * time.Millisecond,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
	})
	c.Start()
	defer c.Stop()

	select {
	case <-reconnected:
	case <-time.After(3 * time.Second):
		t.Fatalf("client did not reconnect after silent stall; connAttempts=%d", connAttempts.Load())
	}

	if got := connAttempts.Load(); got < 2 {
		t.Fatalf("expected >= 2 connection attempts after stall, got %d", got)
	}
}

// TestSSEClientReadTimeoutDoesNotFireOnSteadyKeepalives verifies that a
// stream which keeps emitting bytes within the read window stays alive — the
// deadline is per-read, not absolute. Anchors the "reset on each read"
// behavior so a future refactor doesn't silently make the deadline absolute.
func TestSSEClientReadTimeoutDoesNotFireOnSteadyKeepalives(t *testing.T) {
	var connAttempts atomic.Int32
	stop := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		connAttempts.Add(1)
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_, _ = fmt.Fprint(w, ": keepalive\n\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			case <-stop:
				return
			}
		}
	}))
	defer server.Close()
	defer close(stop)

	c := newSSEClient(sseClientConfig{
		URL:          server.URL,
		APIKey:       "test-key",
		OnEnvelope:   func(*ConfigEnvelope) {},
		ReadTimeout:  150 * time.Millisecond,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
	})
	c.Start()
	defer c.Stop()

	// Hold a window that is ~5x the read deadline. With steady keepalives the
	// deadline should reset on each read; we should see exactly one connection.
	time.Sleep(750 * time.Millisecond)

	if got := connAttempts.Load(); got != 1 {
		t.Fatalf("expected exactly 1 connection attempt with steady keepalives, got %d", got)
	}
}

// TestSSEClientDefaultsToHTTP11 verifies the SSE client's default *http.Client
// is wired with a transport that disables HTTP/2. Toxiproxy is TCP-only, so an
// h2 stream stall (where stream-level frames are silent but connection-level
// frames flow) would be invisible in CI. Forcing HTTP/1.1 on the SSE socket
// keeps stall detection observable. The polling client is unaffected.
func TestSSEClientDefaultsToHTTP11(t *testing.T) {
	c := newSSEClient(sseClientConfig{
		URL:        "http://example.invalid",
		APIKey:     "test-key",
		OnEnvelope: func(*ConfigEnvelope) {},
	})

	tr, ok := c.cfg.Client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport on default SSE client, got %T", c.cfg.Client.Transport)
	}
	if tr.ForceAttemptHTTP2 {
		t.Error("expected ForceAttemptHTTP2=false on default SSE transport")
	}
	if tr.TLSNextProto == nil {
		t.Error("expected TLSNextProto to be a non-nil (empty) map to disable h2 negotiation; nil enables default h2")
	}
	if len(tr.TLSNextProto) != 0 {
		t.Errorf("expected empty TLSNextProto map, got %d entries", len(tr.TLSNextProto))
	}
	// TLSNextProto alone is insufficient: it disables Go's automatic h2
	// RoundTripper dispatch but does NOT prevent the TLS ClientHello from
	// advertising "h2" in ALPN. When a server prefers h2 (Fly's edge does)
	// the negotiation lands on h2, the http.Transport receives raw h2
	// frames on an HTTP/1 socket, and connectOnce errors with
	// "malformed HTTP response \"\\x00\\x00\\x18\\x04...\"" indefinitely.
	// Pinning TLSClientConfig.NextProtos = ["http/1.1"] is what actually
	// removes h2 from the offer (qfg-hpqj).
	if tr.TLSClientConfig == nil {
		t.Fatal("expected TLSClientConfig to be set so ALPN advertises only http/1.1")
	}
	if got, want := tr.TLSClientConfig.NextProtos, []string{"http/1.1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("TLSClientConfig.NextProtos = %v, want %v", got, want)
	}
}

// TestSSEClientNegotiatesHTTP11AgainstH2PreferringServer is the end-to-end
// regression for qfg-hpqj: prior to the fix, the SSE transport's TLS
// ClientHello advertised "h2, http/1.1" in ALPN. A server that prefers h2
// (Fly's TLS edge) would pick h2, send raw HTTP/2 frames over a socket the
// transport tried to parse as HTTP/1, and every connectOnce failed with a
// "malformed HTTP response" error — silently, since failures don't log.
// All existing SSE unit tests use httptest.NewServer (plaintext) so this
// path was uncovered. This test starts a TLS server that ADVERTISES h2 first
// in ALPN and asserts the SSE client's default transport still negotiates
// http/1.1.
func TestSSEClientNegotiatesHTTP11AgainstH2PreferringServer(t *testing.T) {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{NextProtos: []string{"h2", "http/1.1"}}
	srv.StartTLS()
	defer srv.Close()

	c := newSSEClient(sseClientConfig{
		URL:        srv.URL,
		APIKey:     "test-key",
		OnEnvelope: func(*ConfigEnvelope) {},
	})

	tr, ok := c.cfg.Client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport on default SSE client, got %T", c.cfg.Client.Transport)
	}
	// Trust the test cert without disturbing NextProtos — that's the
	// behavior under test.
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	tr.TLSClientConfig.RootCAs = pool

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.cfg.Client.Do(req)
	if err != nil {
		t.Fatalf("request failed (likely h2 negotiated then parsed as h1): %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.Proto != "HTTP/1.1" {
		t.Errorf("negotiated protocol = %q, want HTTP/1.1 (server preferred h2; ALPN must have offered only http/1.1)", resp.Proto)
	}
}

// TestSSEClientDefaultReadTimeoutIs90s anchors the production default. 90s is
// 3x the 30s server heartbeat (one missed heartbeat as noise tolerance, two
// missed as a clear signal). See qfg-47c2.10.
func TestSSEClientDefaultReadTimeoutIs90s(t *testing.T) {
	c := newSSEClient(sseClientConfig{
		URL:        "http://example.invalid",
		APIKey:     "test-key",
		OnEnvelope: func(*ConfigEnvelope) {},
	})
	if got, want := c.cfg.ReadTimeout, 90*time.Second; got != want {
		t.Errorf("default ReadTimeout = %v, want %v", got, want)
	}
}

// TestSSEClientRecoversFromOnEnvelopePanic verifies that a panic raised inside
// the user-supplied OnEnvelope callback does NOT crash the SSE loop. The
// failure mode this guards against: a buggy customer callback (nil deref,
// panic on type assertion, etc.) used to take down the whole process because
// the panic bubbled out of parseStream → connectOnce → runLoop and tore the
// goroutine apart. After qfg-47c2.11 the panic is recovered, logged at ERROR
// with the panic value, the quonfig_sdk_worker_restart_total{reason="callback_panic"}
// counter is incremented, and the loop continues parsing subsequent events.
func TestSSEClientRecoversFromOnEnvelopePanic(t *testing.T) {
	var calls atomic.Int32
	envCh := make(chan *ConfigEnvelope, 4)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		w.WriteHeader(http.StatusOK)
		writeSSEEnvelope(w, flusher, makeEnvelope("v1", "flag.p", "boom"))
		writeSSEEnvelope(w, flusher, makeEnvelope("v2", "flag.p", "after-panic"))
		<-r.Context().Done()
	}))
	defer server.Close()

	var logBuf bytes.Buffer
	var logMu sync.Mutex
	handler := slog.NewTextHandler(&syncWriter{w: &logBuf, mu: &logMu}, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	c := newSSEClient(sseClientConfig{
		URL:    server.URL,
		APIKey: "test-key",
		Logger: logger,
		OnEnvelope: func(env *ConfigEnvelope) {
			n := calls.Add(1)
			envCh <- env
			if n == 1 {
				panic("simulated callback panic")
			}
		},
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
	})
	c.Start()
	defer c.Stop()

	// First envelope (panics)
	select {
	case <-envCh:
	case <-time.After(2 * time.Second):
		t.Fatal("first envelope never delivered to callback")
	}
	// Second envelope must still be delivered — loop must have survived.
	select {
	case env := <-envCh:
		if v := env.Configs[0].Default.Rules[0].Value.Value.(string); v != "after-panic" {
			t.Errorf("second envelope value = %q, want after-panic", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("second envelope never delivered after panic recovery; calls=%d", calls.Load())
	}

	if got := c.workerRestartTotal("callback_panic"); got != 1 {
		t.Errorf("workerRestartTotal(callback_panic) = %d, want 1", got)
	}

	logMu.Lock()
	logOut := logBuf.String()
	logMu.Unlock()
	if !strings.Contains(logOut, "level=ERROR") {
		t.Errorf("expected an ERROR-level log line, got: %s", logOut)
	}
	if !strings.Contains(logOut, "simulated callback panic") {
		t.Errorf("expected the panic value to appear in the log output, got: %s", logOut)
	}
}

// syncWriter wraps an io.Writer with a mutex so the test can safely read while
// slog writes from the SSE goroutine.
type syncWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
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
		// The streamed envelope must carry a higher generation than the polled
		// one so the reject-older guard heals forward to it (production bumps the
		// generation on every change).
		streamedEnv := makeEnvelope("v2", "flag.x", "streamed")
		streamedEnv.Meta.Generation = 2
		writeSSEEnvelope(w, flusher, streamedEnv)
		<-r.Context().Done()
	}))
	defer sseServer.Close()

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "v1")
		polledEnv := makeEnvelope("v1", "flag.x", "polled")
		polledEnv.Meta.Generation = 1
		_ = json.NewEncoder(w).Encode(polledEnv)
	}))
	defer httpServer.Close()

	client, err := NewClient(
		WithSdkKey("test-key"),
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
