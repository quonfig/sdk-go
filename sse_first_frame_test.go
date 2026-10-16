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

// Supervisor contract Test 9 (ITD v2026.10.16.4,
// chaos/supervisor-test-contract.md; plan
// project/plans/2026-10-12-sse-connected-first-frame.md; bead
// qfg-goi1.10.39): a stream connection counts as connected only at its FIRST
// VALID FRAME (a complete comment-only block such as `: keepalive`, or a data
// block that decodes and validates as a config envelope). 200 headers alone
// are not a frame. A connection that ends before its first frame is a failed
// attempt: the backoff grows, no connected edge is reported, and the fallback
// poller's engage timer is not cancelled.
//
// These drive the real SSE worker and the real fallback poller against a
// local 127.0.0.1 server. The config endpoint serves envelope v1 on the init
// fetch and v2 on every later fetch, so a frozen config is observable directly
// through GetStringValue.

const firstFrameKey = "ff.value"

func firstFrameEnvelope(gen int, val string) string {
	g := string(rune('0' + gen))
	return `{"configs":[{"key":"` + firstFrameKey + `","type":"config","valueType":"string","default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"string","value":"` + val + `"}}]}}],"meta":{"version":"gen-` + g + `","environment":"Production","generation":` + g + `}}`
}

type firstFrameRig struct {
	srv        *httptest.Server
	client     *Client
	sseConns   atomic.Int32
	configHits atomic.Int32
	start      time.Time

	mu         sync.Mutex
	edges      []stateEdge
	sseSleeps  []time.Duration
	sawConn    bool // ConnectionState() was ever "connected"
	sawConnAt  time.Time
	sawPoller  bool // FallbackPollerActive() was ever true
	stopSample chan struct{}
}

type stateEdge struct {
	connected bool
	at        time.Time
}

// newFirstFrameRig starts the server with the given SSE handler and a client
// against it. stream is called once per SSE attempt with the attempt number
// (1-based); the 200 + text/event-stream headers are already written and
// flushed when it runs.
func newFirstFrameRig(t *testing.T, threshold time.Duration, stream func(n int32, w http.ResponseWriter, f http.Flusher, r *http.Request)) *firstFrameRig {
	t.Helper()
	rig := &firstFrameRig{stopSample: make(chan struct{})}
	rig.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v2/configs") {
			w.Header().Set("Content-Type", "application/json")
			if rig.configHits.Add(1) == 1 {
				_, _ = w.Write([]byte(firstFrameEnvelope(1, "v1")))
				return
			}
			_, _ = w.Write([]byte(firstFrameEnvelope(2, "v2")))
			return
		}
		n := rig.sseConns.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		f.Flush()
		stream(n, w, f, r)
	}))
	t.Cleanup(rig.srv.Close)

	rig.start = time.Now()
	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{rig.srv.URL}),
		WithSSE(true),
		withTestStreamURLOverride(rig.srv.URL+"/api/v2/sse/config"),
		WithFallbackPoll(true, 20*time.Millisecond),
		withTestFallbackPollThreshold(threshold),
		WithSSEStateCallback(func(v bool) {
			rig.mu.Lock()
			rig.edges = append(rig.edges, stateEdge{connected: v, at: time.Now()})
			rig.mu.Unlock()
		}),
		withTestWaitHook(func(kind string, d time.Duration) time.Duration {
			if kind == "sse" {
				rig.mu.Lock()
				rig.sseSleeps = append(rig.sseSleeps, d)
				rig.mu.Unlock()
			}
			return d
		}),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	rig.client = client
	if v, _, _ := client.GetStringValue(firstFrameKey, nil); v != "v1" {
		t.Fatalf("init value = %q, want v1", v)
	}

	// Sample the public state surfaces continuously, so a brief connected
	// flap between two observations still counts.
	go func() {
		for {
			select {
			case <-rig.stopSample:
				return
			case <-time.After(time.Millisecond):
			}
			state := client.ConnectionState()
			active := client.FallbackPollerActive()
			rig.mu.Lock()
			if state == ConnStateConnected && !rig.sawConn {
				rig.sawConn = true
				rig.sawConnAt = time.Now()
			}
			if active {
				rig.sawPoller = true
			}
			rig.mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		close(rig.stopSample)
		client.Close()
	})
	return rig
}

func (r *firstFrameRig) connectedEdges() []time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []time.Time
	for _, e := range r.edges {
		if e.connected {
			out = append(out, e.at)
		}
	}
	return out
}

func (r *firstFrameRig) sleeps() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.sseSleeps...)
}

func (r *firstFrameRig) value() string {
	v, _, _ := r.client.GetStringValue(firstFrameKey, nil)
	return v
}

func sleepUntil(t time.Time) {
	if d := time.Until(t); d > 0 {
		time.Sleep(d)
	}
}

// 9a — headers only: every attempt is 200 + text/event-stream, flush, close.
func TestSupervisor_FirstFrame9a_HeadersOnlyIsAFailedAttempt(t *testing.T) {
	// The threshold (1s) is longer than the longest reconnect sleep at the
	// initial delay (500ms). With the backoff reset and the engage timer
	// cancelled on every 200, the stream is never "down" that long, so the
	// poller never engages and config stays frozen at v1. Only treating the
	// frameless 200 as a failed attempt lets the engage timer run out.
	rig := newFirstFrameRig(t, time.Second, func(_ int32, _ http.ResponseWriter, _ http.Flusher, _ *http.Request) {})

	// Anti-vacuity: assert through the value the fallback poller delivers.
	waitFor(t, 3*time.Second, func() bool { return rig.client.FallbackPollerActive() && rig.value() == "v2" },
		"fallback poller never delivered v2: frameless 200s kept cancelling the engage timer (config frozen at v1)")
	if got := rig.client.ConnectionState(); got != ConnStateFallingBack {
		t.Errorf("ConnectionState = %q, want %q", got, ConnStateFallingBack)
	}

	// Ladder bound: requested sleeps are >= 250ms, 500ms, 1s, 2s (Test 2), so
	// at most 4 attempts fit in 3s. A backoff reset on every 200 allows ~10.
	sleepUntil(rig.start.Add(3 * time.Second))
	if n := rig.sseConns.Load(); n > 4 {
		t.Errorf("SSE attempts in 3s = %d, want <= 4 (backoff reset on a frameless 200)", n)
	}
	for i, s := range rig.sleeps() {
		floor := (250 * time.Millisecond) << i
		if floor > 15*time.Second {
			floor = 15 * time.Second
		}
		if s < floor {
			t.Errorf("requested SSE sleep #%d = %s, want >= %s (backoff must grow after a frameless 200)", i, s, floor)
		}
	}
	if edges := rig.connectedEdges(); len(edges) != 0 {
		t.Errorf("connected edges = %d, want 0 (200 headers alone are not a frame)", len(edges))
	}
	rig.mu.Lock()
	sawConn := rig.sawConn
	rig.mu.Unlock()
	if sawConn {
		t.Errorf("ConnectionState() was %q at some point; a frameless 200 must never report it", ConnStateConnected)
	}
}

// 9b — headers early, frame late: 200 + headers, flush, wait 1s, then
// `: keepalive`, hold the stream open.
func TestSupervisor_FirstFrame9b_ConnectedAtFirstFrameNotHeaders(t *testing.T) {
	var sentAt atomic.Int64
	rig := newFirstFrameRig(t, 10*time.Second, func(_ int32, w http.ResponseWriter, f http.Flusher, r *http.Request) {
		select {
		case <-time.After(time.Second):
		case <-r.Context().Done():
			return
		}
		sentAt.Store(time.Now().UnixNano())
		_, _ = w.Write([]byte(": keepalive\n\n"))
		f.Flush()
		<-r.Context().Done()
	})

	waitFor(t, 3*time.Second, func() bool { return sentAt.Load() != 0 }, "server never sent the keepalive")
	sent := time.Unix(0, sentAt.Load())
	waitFor(t, 2*time.Second, func() bool { return rig.client.ConnectionState() == ConnStateConnected },
		"ConnectionState never became connected after the keepalive")
	time.Sleep(500 * time.Millisecond)

	edges := rig.connectedEdges()
	if len(edges) != 1 {
		t.Fatalf("connected edges = %d, want exactly 1", len(edges))
	}
	if early := sent.Sub(edges[0]); early > 50*time.Millisecond {
		t.Errorf("connected edge reported %s before the first frame was sent; want at or after it (>= send - 50ms)", early)
	}
	rig.mu.Lock()
	sawConnAt := rig.sawConnAt
	rig.mu.Unlock()
	if early := sent.Sub(sawConnAt); early > 50*time.Millisecond {
		t.Errorf("ConnectionState() was connected %s before the first frame was sent", early)
	}
	if n := rig.sseConns.Load(); n != 1 {
		t.Errorf("SSE attempts = %d, want 1", n)
	}
}

// 9c — one keepalive then close, every attempt. A keepalive is a valid frame:
// each attempt connects, the backoff stays at the initial delay, and the
// poller does not engage within the threshold.
func TestSupervisor_FirstFrame9c_KeepaliveThenCloseResetsBackoff(t *testing.T) {
	rig := newFirstFrameRig(t, time.Second, func(_ int32, w http.ResponseWriter, f http.Flusher, _ *http.Request) {
		_, _ = w.Write([]byte(": keepalive\n\n"))
		f.Flush()
	})

	sleepUntil(rig.start.Add(2500 * time.Millisecond))
	conns := rig.sseConns.Load()
	edges := rig.connectedEdges()
	if conns < 3 {
		t.Fatalf("SSE attempts in 2.5s = %d, want >= 3 (backoff should stay at the initial delay)", conns)
	}
	if n := int32(len(edges)); n < conns-1 || n > conns {
		t.Errorf("connected edges = %d for %d attempts, want one per attempt", n, conns)
	}
	for i, s := range rig.sleeps() {
		if s > 500*time.Millisecond {
			t.Errorf("requested SSE sleep #%d = %s, want <= 500ms (a session that delivered a frame resets the backoff)", i, s)
		}
	}
	rig.mu.Lock()
	sawPoller := rig.sawPoller
	rig.mu.Unlock()
	if sawPoller {
		t.Errorf("fallback poller engaged although every attempt delivered a keepalive frame")
	}
	if v := rig.value(); v != "v1" {
		t.Errorf("value = %q, want v1 (no poll should have run)", v)
	}
}

// 9d — control: 200 + a snapshot envelope, hold the stream open.
func TestSupervisor_FirstFrame9d_SnapshotConnects(t *testing.T) {
	rig := newFirstFrameRig(t, 10*time.Second, func(_ int32, w http.ResponseWriter, f http.Flusher, r *http.Request) {
		_, _ = w.Write([]byte("id: gen-2\ndata: " + firstFrameEnvelope(2, "v2") + "\n\n"))
		f.Flush()
		<-r.Context().Done()
	})

	waitFor(t, 3*time.Second, func() bool {
		return rig.client.ConnectionState() == ConnStateConnected && rig.value() == "v2"
	}, "snapshot envelope did not connect and install v2")
	time.Sleep(200 * time.Millisecond)
	if edges := rig.connectedEdges(); len(edges) != 1 {
		t.Errorf("connected edges = %d, want 1", len(edges))
	}
	if rig.client.FallbackPollerActive() {
		t.Errorf("fallback poller engaged on a healthy stream")
	}
	if n := rig.configHits.Load(); n != 1 {
		t.Errorf("config fetches = %d, want 1 (init only)", n)
	}
}

// Test 2 reset amendment: a session that delivered a frame resets the
// backoff; a 200 that ends with no frame does not (the backoff grows).
func TestSupervisor_BackoffResetRequiresFirstFrame(t *testing.T) {
	var conns atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := conns.Add(1)
		switch n {
		case 1, 3:
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		f.Flush()
		switch n {
		case 2:
			writeSSEEnvelope(w, f, makeEnvelope("v1", "k", "v"))
		case 4:
			// 200 + headers, then close with no frame.
		default:
			<-r.Context().Done()
		}
	}))
	t.Cleanup(srv.Close)

	var mu sync.Mutex
	var sleeps []time.Duration
	c := newSSEClient(sseClientConfig{
		URL:        srv.URL,
		APIKey:     "test-key",
		OnEnvelope: func(*ConfigEnvelope) {},
		Wait: func(kind string, d time.Duration) time.Duration {
			mu.Lock()
			sleeps = append(sleeps, d)
			mu.Unlock()
			return time.Millisecond
		},
	})
	c.Start()
	t.Cleanup(c.Stop)

	waitFor(t, 3*time.Second, func() bool { return conns.Load() >= 5 }, "client did not reach the fifth attempt")
	mu.Lock()
	got := append([]time.Duration(nil), sleeps...)
	mu.Unlock()
	if len(got) < 4 {
		t.Fatalf("recorded %d sleeps, want >= 4", len(got))
	}
	// Base delays: 500ms (fail), 500ms (reset by the delivered frame),
	// 500ms (fail), then 1s: the frameless 200 must not reset. Requested
	// sleeps are in [base/2, base].
	if got[1] > 500*time.Millisecond {
		t.Errorf("sleep after a session with a frame = %s, want <= 500ms (reset)", got[1])
	}
	if got[3] < 500*time.Millisecond {
		t.Errorf("sleep after a frameless 200 = %s, want >= 500ms (grown from 1s base, not reset)", got[3])
	}
}

// Frame classification for the first-frame rule: what counts as a valid
// frame and what does not.
func TestSSEParseStreamFirstFrameClassification(t *testing.T) {
	env := "data: " + firstFrameEnvelope(2, "v2") + "\n\n"
	cases := []struct {
		name   string
		stream string
		want   bool
	}{
		{"empty body (headers only)", "", false},
		{"keepalive cut off by EOF", ": keepalive\n", false},
		{"complete keepalive", ": keepalive\n\n", true},
		{"malformed data block", "data: {not json\n\n", false},
		{"non-envelope JSON", "data: {\"hello\":\"world\"}\n\n", false},
		{"envelope cut off by EOF", "data: " + firstFrameEnvelope(2, "v2") + "\n", false},
		{"id-only block", "id: gen-2\n\n", false},
		{"bare blank line", "\n", false},
		{"valid envelope", env, true},
		{"garbage then keepalive", "data: nope\n\n: keepalive\n\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var edges []bool
			c := newSSEClient(sseClientConfig{
				URL:                   "http://127.0.0.1:1",
				OnEnvelope:            func(*ConfigEnvelope) {},
				OnStateChangeInternal: func(v bool) { edges = append(edges, v) },
			})
			got, err := c.parseStream(strings.NewReader(tc.stream))
			if err != nil {
				t.Fatalf("parseStream err = %v", err)
			}
			if got != tc.want {
				t.Errorf("frameSeen = %v, want %v", got, tc.want)
			}
			if wantEdges := tc.want; (len(edges) == 1 && edges[0]) != wantEdges || len(edges) > 1 {
				t.Errorf("connected edges = %v, want one true edge iff a frame was seen (%v)", edges, tc.want)
			}
		})
	}
}
