package quonfig

// Telemetry transport contract tests T1-T8 (qfg-y8je.6).
//
// Spec: integration-test-data/chaos/telemetry-transport-contract.md, policy
// P1-P10 in project/plans/2026-09-24-sdk-telemetry-transport-policy.md. The
// reference implementation is sdk-node test/telemetry-transport.test.ts.
//
// Fixture (per the contract): a real httptest server scripted per received
// POST (status, Retry-After, hang), a manual clock injected through the
// test-only Options.testTelemetryClock seam (it also drives the per-POST
// deadline), and a capturing slog handler at DEBUG. Evaluations go through the
// public GetStringValue API on a datadir client with an SDK key, so every
// batch is a real payload. Nothing in the reporter or its queue is mocked.

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quonfig/sdk-go/internal/telemetry"
)

// ---------------------------------------------------------------------------
// Manual clock
// ---------------------------------------------------------------------------

type manualTimer struct {
	c       *manualClock
	id      int
	due     time.Time
	f       func()
	stopped bool
}

func (t *manualTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	for i, x := range t.c.timers {
		if x == t {
			t.c.timers = append(t.c.timers[:i], t.c.timers[i+1:]...)
			break
		}
	}
	return true
}

// manualClock fires due timers in time order, each on its own goroutine like
// time.AfterFunc, and after each one waits for the SDK to settle (the tick
// finished, or it is blocked on a POST the stub is holding).
type manualClock struct {
	mu      sync.Mutex
	now     time.Time
	timers  []*manualTimer
	nextID  int
	running atomic.Int64
	settle  func()
}

func newManualClock() *manualClock {
	// Whole seconds, so an HTTP-date Retry-After maps exactly.
	return &manualClock{now: time.Unix(1_800_000_000, 0)}
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) AfterFunc(d time.Duration, f func()) telemetry.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	t := &manualTimer{c: c, id: c.nextID, due: c.now.Add(d), f: f}
	c.timers = append(c.timers, t)
	return t
}

func (c *manualClock) WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	t := c.AfterFunc(d, func() { cancel(context.DeadlineExceeded) })
	return ctx, func() {
		t.Stop()
		cancel(context.Canceled)
	}
}

// Advance moves the clock forward by d, firing every timer that falls due.
func (c *manualClock) Advance(d time.Duration) {
	c.mu.Lock()
	target := c.now.Add(d)
	c.mu.Unlock()
	for {
		c.mu.Lock()
		var next *manualTimer
		for _, t := range c.timers {
			if !t.due.After(target) && (next == nil || t.due.Before(next.due) || (t.due.Equal(next.due) && t.id < next.id)) {
				next = t
			}
		}
		if next == nil {
			c.now = target
			c.mu.Unlock()
			return
		}
		c.now = next.due
		next.stopped = true
		for i, x := range c.timers {
			if x == next {
				c.timers = append(c.timers[:i], c.timers[i+1:]...)
				break
			}
		}
		c.mu.Unlock()

		c.running.Add(1)
		go func(f func()) {
			defer c.running.Add(-1)
			f()
		}(next.f)
		if c.settle != nil {
			c.settle()
		}
	}
}

// ---------------------------------------------------------------------------
// Scriptable telemetry stub
// ---------------------------------------------------------------------------

type stubStep struct {
	status     int
	retryAfter string
	hang       bool
}

type telemetryStub struct {
	srv      *httptest.Server
	mu       sync.Mutex
	bodies   [][]byte
	script   []stubStep
	def      stubStep
	holding  int
	releases map[int]chan stubStep
	closing  chan struct{}
}

func newTelemetryStub(t *testing.T) *telemetryStub {
	t.Helper()
	s := &telemetryStub{def: stubStep{status: 200}, releases: map[int]chan stubStep{}, closing: make(chan struct{})}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		i := len(s.bodies)
		s.bodies = append(s.bodies, body)
		step := s.def
		if len(s.script) > 0 {
			step = s.script[0]
			s.script = s.script[1:]
		}
		var rel chan stubStep
		if step.hang {
			rel = make(chan stubStep, 1)
			s.releases[i] = rel
			s.holding++
		}
		s.mu.Unlock()

		if step.hang {
			defer func() {
				s.mu.Lock()
				s.holding--
				s.mu.Unlock()
			}()
			select {
			case st := <-rel:
				step = st
			case <-r.Context().Done():
				return
			case <-s.closing:
				return
			}
		}
		if step.retryAfter != "" {
			w.Header().Set("Retry-After", step.retryAfter)
		}
		w.WriteHeader(step.status)
		_, _ = fmt.Fprintf(w, "stub %d", step.status)
	}))
	return s
}

func (s *telemetryStub) Script(steps ...stubStep) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.script = append(s.script, steps...)
}

func (s *telemetryStub) SetDefault(step stubStep) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.def = step
}

func (s *telemetryStub) PostCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bodies)
}

func (s *telemetryStub) Body(i int) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bodies[i]
}

func (s *telemetryStub) Sha(i int) [32]byte { return sha256.Sum256(s.Body(i)) }

func (s *telemetryStub) Holding() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.holding
}

func (s *telemetryStub) Release(i int, step stubStep) {
	s.mu.Lock()
	rel := s.releases[i]
	s.mu.Unlock()
	rel <- step
}

// ---------------------------------------------------------------------------
// Capturing logger
// ---------------------------------------------------------------------------

type capturedLine struct {
	level slog.Level
	msg   string
}

type captureHandler struct {
	mu    *sync.Mutex
	lines *[]capturedLine
}

func (h captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.lines = append(*h.lines, capturedLine{level: r.Level, msg: r.Message})
	return nil
}
func (h captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h captureHandler) WithGroup(string) slog.Handler      { return h }

type captureLogger struct {
	mu    sync.Mutex
	lines []capturedLine
}

func (c *captureLogger) Logger() *slog.Logger {
	return slog.New(captureHandler{mu: &c.mu, lines: &c.lines})
}

// LogCount counts telemetry lines at exactly that level whose message matches re.
func (c *captureLogger) LogCount(level slog.Level, re string) int {
	rx := regexp.MustCompile(re)
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, l := range c.lines {
		if l.level == level && rx.MatchString(l.msg) {
			n++
		}
	}
	return n
}

func (c *captureLogger) Dump() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var b strings.Builder
	for _, l := range c.lines {
		fmt.Fprintf(&b, "  %s %s\n", l.level, l.msg)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

const transportFixtureConfigs = 60

func transportWorkspace(t *testing.T) string {
	t.Helper()
	files := map[string]string{}
	for i := 0; i < transportFixtureConfigs; i++ {
		key := fmt.Sprintf("cfg-%02d", i)
		files["configs/"+key+".json"] = fmt.Sprintf(`{
			"id":"%s-id","key":"%s","type":"config","valueType":"string","sendToClientSdk":false,
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"string","value":"v-%s"}}]},
			"environments":[]
		}`, key, key, key)
	}
	return writeWorkspaceFiles(t, `{"prod":"Production"}`, files)
}

type transportHarness struct {
	t      *testing.T
	stub   *telemetryStub
	clock  *manualClock
	logs   *captureLogger
	client *Client
	sub    *telemetry.Submitter
	closed bool
}

func newTransportHarness(t *testing.T, opts ...Option) *transportHarness {
	t.Helper()
	t.Setenv("QUONFIG_BACKEND_SDK_KEY", "")
	h := &transportHarness{t: t, stub: newTelemetryStub(t), clock: newManualClock(), logs: &captureLogger{}}
	h.clock.settle = h.settle

	all := []Option{
		WithSdkKey("test-sdk-key"),
		WithDataDir(transportWorkspace(t)),
		WithEnvironment("Production"),
		WithTelemetryURL(h.stub.srv.URL),
		WithLogger(h.logs.Logger()),
		WithQuonfigUserContext(false),
		func(o *Options) error { o.testTelemetryClock = h.clock; return nil },
	}
	all = append(all, opts...)
	client, err := NewClient(all...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.telemetry == nil {
		t.Fatal("telemetry did not start")
	}
	h.client = client
	h.sub = client.telemetry.submitter

	t.Cleanup(func() {
		close(h.stub.closing) // release any hung handler
		if !h.closed {
			done := make(chan struct{})
			go func() { client.Close(); close(done) }()
			// A final flush against the stub answers promptly now; the deadline
			// is on the manual clock, so nudge it in case it is still waiting.
			for i := 0; i < 50; i++ {
				select {
				case <-done:
					i = 50
				case <-time.After(20 * time.Millisecond):
					h.clock.Advance(time.Second)
				}
			}
		}
		h.stub.srv.Close()
	})
	return h
}

// settle waits (real time) until every fired clock callback has finished, or
// the only one left is a tick blocked on a POST that the stub is holding.
func (h *transportHarness) settle() {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		running := h.clock.running.Load()
		if running == 0 {
			return
		}
		st := h.sub.DebugState()
		if running == 1 && st.PostInFlight && !st.PostCanceled && h.stub.Holding() >= 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	h.t.Fatalf("SDK did not settle: running=%d state=%+v holding=%d", h.clock.running.Load(), h.sub.DebugState(), h.stub.Holding())
}

func (h *transportHarness) advance(d time.Duration) { h.clock.Advance(d) }

// record evaluates set k through the public API and waits until the
// evaluations reached the aggregators (recording is asynchronous).
func (h *transportHarness) record(k int) {
	h.t.Helper()
	for j := 0; j < 2; j++ {
		key := fmt.Sprintf("cfg-%02d", (2*k+j)%transportFixtureConfigs)
		ctx := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"key": fmt.Sprintf("set-%d", k)})
		if _, ok, err := h.client.GetStringValue(key, ctx); err != nil || !ok {
			h.t.Fatalf("GetStringValue(%s) = %v, %v", key, ok, err)
		}
	}
	h.waitRecorded()
}

func (h *transportHarness) waitRecorded() {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for h.sub.PendingRecords() > 0 {
		if time.Now().After(deadline) {
			h.t.Fatal("recorded evaluations never reached the aggregators")
		}
		time.Sleep(time.Millisecond)
	}
}

func (h *transportHarness) waitPosts(n int) {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for h.stub.PostCount() < n {
		if time.Now().After(deadline) {
			h.t.Fatalf("stub saw %d POSTs, want %d", h.stub.PostCount(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

func (h *transportHarness) waitIdle() {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for h.clock.running.Load() > 0 || h.sub.DebugState().TickRunning {
		if time.Now().After(deadline) {
			h.t.Fatal("tick never finished")
		}
		time.Sleep(time.Millisecond)
	}
}

func (h *transportHarness) state() telemetry.DebugState { return h.sub.DebugState() }

// sets returns the evaluation sets ("set-k" user keys) a body carries, sorted.
func bodySets(t *testing.T, body []byte) []int {
	t.Helper()
	var payload struct {
		Events []struct {
			Summaries *struct {
				Summaries []struct {
					Key string `json:"key"`
				} `json:"summaries"`
			} `json:"summaries"`
		} `json:"events"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	seen := map[int]bool{}
	for _, ev := range payload.Events {
		if ev.Summaries == nil {
			continue
		}
		for _, s := range ev.Summaries.Summaries {
			var n int
			if _, err := fmt.Sscanf(s.Key, "cfg-%d", &n); err == nil {
				seen[n/2] = true
			}
		}
	}
	out := make([]int, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

const (
	lvDebug = slog.LevelDebug
	lvInfo  = slog.LevelInfo
	lvWarn  = slog.LevelWarn
	lvError = slog.LevelError
)

// ---------------------------------------------------------------------------
// T1 - Timeout aborts and retains (P1, P5, P7)
// ---------------------------------------------------------------------------

func TestTelemetryTransport_T1_TimeoutAbortsAndRetains(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.Script(stubStep{hang: true}, stubStep{status: 200})
	h.record(1)

	h.advance(60 * time.Second) // tick 1: POST 0 hangs
	h.waitPosts(1)
	h.advance(15 * time.Second) // the 15s deadline aborts it
	h.waitIdle()

	if got := h.stub.PostCount(); got != 1 {
		t.Fatalf("post_count = %d, want 1", got)
	}
	if got := h.state().RetainedCount; got != 1 {
		t.Fatalf("retained_count = %d, want 1", got)
	}
	if h.logs.LogCount(lvWarn, ".*") != 0 || h.logs.LogCount(lvError, ".*") != 0 {
		t.Fatalf("timeout logged above DEBUG:\n%s", h.logs.Dump())
	}
	if h.logs.LogCount(lvDebug, ".*") < 1 {
		t.Fatalf("failed POST not logged at DEBUG:\n%s", h.logs.Dump())
	}

	h.advance(45 * time.Second) // tick 2, 45s after the failure
	h.waitIdle()
	if got := h.stub.PostCount(); got != 2 {
		t.Fatalf("post_count = %d, want 2", got)
	}
	if h.stub.Sha(1) != h.stub.Sha(0) {
		t.Fatal("resend is not byte-identical to the timed-out POST")
	}
	if got := h.state().RetainedCount; got != 0 {
		t.Fatalf("retained_count = %d, want 0", got)
	}
	if got := h.logs.LogCount(lvInfo, "(?i)recover"); got != 1 {
		t.Fatalf("recovery INFO count = %d, want 1:\n%s", got, h.logs.Dump())
	}
	if got := h.logs.LogCount(lvWarn, ".*"); got != 0 {
		t.Fatalf("WARN count = %d, want 0:\n%s", got, h.logs.Dump())
	}
}

func TestTelemetryTransport_T1_Defaults(t *testing.T) {
	o := defaultOptions()
	if o.TelemetryTimeout != 15000*time.Millisecond {
		t.Errorf("TelemetryTimeout default = %v, want 15s", o.TelemetryTimeout)
	}
	if o.TelemetryConnectTimeout != 5000*time.Millisecond {
		t.Errorf("TelemetryConnectTimeout default = %v, want 5s", o.TelemetryConnectTimeout)
	}
	if o.TelemetrySyncInterval != 60*time.Second {
		t.Errorf("TelemetrySyncInterval default = %v, want 60s", o.TelemetrySyncInterval)
	}
	if o.ContextTelemetryMode != ContextTelemetryPeriodicExample {
		t.Errorf("ContextTelemetryMode default = %q, want periodic_example", o.ContextTelemetryMode)
	}

	// Resolved on a real submitter with no overrides.
	h := newTransportHarness(t)
	cfg := h.sub.ResolvedConfig()
	if cfg.Timeout != 15*time.Second || cfg.ConnectTimeout != 5*time.Second || cfg.SyncInterval != 60*time.Second {
		t.Errorf("resolved timeout/connect/interval = %v/%v/%v, want 15s/5s/60s", cfg.Timeout, cfg.ConnectTimeout, cfg.SyncInterval)
	}
	// The default telemetry client enforces the 5s connect + TLS deadline.
	tr, ok := h.sub.HTTPClient().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("default telemetry transport is %T, want *http.Transport", h.sub.HTTPClient().Transport)
	}
	if tr.TLSHandshakeTimeout != 5*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 5s", tr.TLSHandshakeTimeout)
	}
}

// A caller-supplied HTTP client (WithHTTPClient) with no timeout, or a longer
// one, cannot remove the 15s policy deadline: it rides the request context.
func TestTelemetryTransport_T1_CustomHTTPClientKeepsDeadline(t *testing.T) {
	h := newTransportHarness(t, WithHTTPClient(&http.Client{}))
	h.stub.Script(stubStep{hang: true})
	h.record(1)
	h.advance(60 * time.Second)
	h.waitPosts(1)
	h.advance(15 * time.Second)
	h.waitIdle()
	if got := h.state().RetainedCount; got != 1 {
		t.Fatalf("retained_count = %d, want 1 (POST not aborted at 15s)", got)
	}
	if h.state().PostInFlight {
		t.Fatal("POST still in flight after the 15s deadline")
	}
}

// ---------------------------------------------------------------------------
// T2 - 5xx retains verbatim and resends (P4, P5)
// ---------------------------------------------------------------------------

func TestTelemetryTransport_T2_5xxRetainsVerbatimAndResends(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.Script(stubStep{status: 503}, stubStep{status: 503}, stubStep{status: 200}, stubStep{status: 200})

	h.record(1) // A
	h.advance(60 * time.Second)
	if got := h.state().RetainedCount; got != 1 {
		t.Fatalf("after tick 1 retained_count = %d, want 1", got)
	}
	h.record(2) // B
	h.advance(60 * time.Second)
	if got := h.state().RetainedCount; got != 2 {
		t.Fatalf("after tick 2 retained_count = %d, want 2", got)
	}
	h.advance(60 * time.Second)

	if got := h.stub.PostCount(); got != 4 {
		t.Fatalf("post_count = %d, want 4", got)
	}
	if h.stub.Sha(0) != h.stub.Sha(1) || h.stub.Sha(1) != h.stub.Sha(2) {
		t.Fatal("bodies 0, 1, 2 are not byte-identical")
	}
	if got := bodySets(t, h.stub.Body(0)); !sameInts(got, []int{1}) {
		t.Fatalf("body(0) sets = %v, want [1]", got)
	}
	if got := bodySets(t, h.stub.Body(3)); !sameInts(got, []int{2}) {
		t.Fatalf("body(3) sets = %v, want [2] (B only, never merged with A)", got)
	}
	if got := h.state().RetainedCount; got != 0 {
		t.Fatalf("retained_count = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// T3 - Non-retryable 4xx (P3)
// ---------------------------------------------------------------------------

func TestTelemetryTransport_T3a_AuthDisables(t *testing.T) {
	for _, status := range []int{401, 403, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			h := newTransportHarness(t)
			h.stub.Script(stubStep{status: 503}, stubStep{status: status})
			h.record(1)
			h.advance(60 * time.Second) // 503: retained
			h.record(2)
			h.advance(60 * time.Second) // status: disable

			if got := h.logs.LogCount(lvError, fmt.Sprint(status)); got != 1 {
				t.Fatalf("ERROR /%d/ count = %d, want 1:\n%s", status, got, h.logs.Dump())
			}
			st := h.state()
			if st.Enabled {
				t.Fatal("telemetry_enabled() = true, want false")
			}
			if st.RetainedCount != 0 {
				t.Fatalf("retained_count = %d, want 0", st.RetainedCount)
			}
			if got := h.logs.LogCount(lvWarn, ".*"); got != 0 {
				t.Fatalf("WARN count = %d, want 0:\n%s", got, h.logs.Dump())
			}
			posts := h.stub.PostCount()
			for i := 3; i < 6; i++ {
				h.record(i)
				h.advance(60 * time.Second)
			}
			if got := h.stub.PostCount(); got != posts {
				t.Fatalf("post_count grew from %d to %d after disable", posts, got)
			}
			if got := h.logs.LogCount(lvError, ".*"); got != 1 {
				t.Fatalf("ERROR count = %d, want 1", got)
			}
		})
	}
}

func TestTelemetryTransport_T3b_PayloadRejectDropsBatch(t *testing.T) {
	for _, status := range []int{400, 413, 422} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			h := newTransportHarness(t)
			h.stub.Script(stubStep{status: status}, stubStep{status: 200})
			h.record(1)
			h.advance(60 * time.Second)

			st := h.state()
			if st.RetainedCount != 0 {
				t.Fatalf("retained_count = %d, want 0", st.RetainedCount)
			}
			if got := h.logs.LogCount(lvError, ".*"); got != 1 {
				t.Fatalf("ERROR count = %d, want 1:\n%s", got, h.logs.Dump())
			}
			if got := h.logs.LogCount(lvWarn, ".*"); got != 0 {
				t.Fatalf("WARN count = %d, want 0:\n%s", got, h.logs.Dump())
			}
			if !st.Enabled {
				t.Fatal("telemetry_enabled() = false, want true")
			}
			h.record(2)
			h.advance(60 * time.Second)
			if got := h.stub.PostCount(); got != 2 {
				t.Fatalf("post_count = %d, want 2", got)
			}
			if h.stub.Sha(0) == h.stub.Sha(1) {
				t.Fatal("body(1) equals body(0): the rejected batch was resent")
			}
		})
	}
}

func TestTelemetryTransport_T3_408IsRetryable(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.Script(stubStep{status: 408})
	h.record(1)
	h.advance(60 * time.Second)
	st := h.state()
	if st.RetainedCount != 1 || !st.Enabled {
		t.Fatalf("408: retained_count=%d enabled=%v, want 1/true", st.RetainedCount, st.Enabled)
	}
}

// ---------------------------------------------------------------------------
// T4 - Retry-After and the 30s floor (P4)
// ---------------------------------------------------------------------------

func TestTelemetryTransport_T4a_30sFloor(t *testing.T) {
	h := newTransportHarness(t, WithTelemetrySyncInterval(8*time.Second))
	h.stub.Script(stubStep{status: 503}, stubStep{status: 200})
	h.record(1)
	h.advance(8 * time.Second) // POST 0 fails at F
	if got := h.stub.PostCount(); got != 1 {
		t.Fatalf("post_count = %d, want 1", got)
	}
	for i := 1; i <= 3; i++ { // F+8, F+16, F+24
		h.advance(8 * time.Second)
		if got := h.stub.PostCount(); got != 1 {
			t.Fatalf("tick at F+%ds sent: post_count = %d, want 1", 8*i, got)
		}
	}
	h.advance(8 * time.Second) // F+32
	if got := h.stub.PostCount(); got != 2 {
		t.Fatalf("tick at F+32s: post_count = %d, want 2", got)
	}
	if h.stub.Sha(1) != h.stub.Sha(0) {
		t.Fatal("resend differs from body(0)")
	}
}

func TestTelemetryTransport_T4b_RetryAfterHonored(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.Script(stubStep{status: 429, retryAfter: "120"}, stubStep{status: 200})
	h.record(1)
	h.advance(60 * time.Second) // F = 60s
	h.advance(59 * time.Second) // F+59 (no tick due)
	h.advance(60 * time.Second) // F+119: tick at F+60 fired and was blocked
	if got := h.stub.PostCount(); got != 1 {
		t.Fatalf("post_count at F+119s = %d, want 1", got)
	}
	h.advance(1 * time.Second) // F+120: tick fires
	if got := h.stub.PostCount(); got != 2 {
		t.Fatalf("post_count at F+120s = %d, want 2", got)
	}
	if h.stub.Sha(1) != h.stub.Sha(0) {
		t.Fatal("resend differs from body(0)")
	}
}

func TestTelemetryTransport_T4c_RetryAfterClampedTo600s(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.Script(stubStep{status: 503, retryAfter: "3600"}, stubStep{status: 200})
	h.record(1)
	h.advance(60 * time.Second) // F
	h.record(2)
	h.advance(599 * time.Second) // F+599
	if got := h.stub.PostCount(); got != 1 {
		t.Fatalf("post_count at F+599s = %d, want 1", got)
	}
	h.advance(1 * time.Second) // F+600
	if got := h.stub.PostCount(); got != 2 {
		t.Fatalf("post_count at F+600s = %d, want 2", got)
	}
	if got := bodySets(t, h.stub.Body(1)); !sameInts(got, []int{2}) {
		t.Fatalf("body(1) sets = %v, want [2] (aged batch discarded)", got)
	}
	if got := h.logs.LogCount(lvWarn, ".*"); got != 1 {
		t.Fatalf("WARN count = %d, want 1 (the age discard):\n%s", got, h.logs.Dump())
	}
}

func TestTelemetryTransport_T4d_RetryAfterHTTPDate(t *testing.T) {
	h := newTransportHarness(t)
	h.record(1)
	date := h.clock.Now().Add(60 * time.Second).Add(120 * time.Second).UTC().Format(http.TimeFormat)
	h.stub.Script(stubStep{status: 503, retryAfter: date}, stubStep{status: 200})
	h.advance(60 * time.Second)  // F
	h.advance(119 * time.Second) // F+119
	if got := h.stub.PostCount(); got != 1 {
		t.Fatalf("post_count at F+119s = %d, want 1", got)
	}
	h.advance(1 * time.Second)
	if got := h.stub.PostCount(); got != 2 {
		t.Fatalf("post_count at F+120s = %d, want 2", got)
	}
}

// ---------------------------------------------------------------------------
// T5 - Caps under outage (P5, P6)
// ---------------------------------------------------------------------------

func TestTelemetryTransport_T5_QueueCaps(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.SetDefault(stubStep{status: 503})
	firstSha := map[int][32]byte{}
	for k := 1; k <= 8; k++ {
		h.record(k)
		before := h.stub.PostCount()
		h.advance(60 * time.Second)
		for i := before; i < h.stub.PostCount(); i++ {
			sets := bodySets(t, h.stub.Body(i))
			if len(sets) == 1 {
				if _, ok := firstSha[sets[0]]; !ok {
					firstSha[sets[0]] = h.stub.Sha(i)
				}
			}
		}
		st := h.state()
		if st.RetainedCount > 5 || st.RetainedBytes > 2097152 {
			t.Fatalf("tick %d: retained %d batches / %d bytes, over the cap", k, st.RetainedCount, st.RetainedBytes)
		}
	}
	if got := h.state().RetainedCount; got != 5 {
		t.Fatalf("after tick 8 retained_count = %d, want 5", got)
	}

	h.stub.SetDefault(stubStep{status: 200})
	before := h.stub.PostCount()
	h.advance(60 * time.Second) // tick 9, nothing new recorded
	if got := h.stub.PostCount() - before; got != 5 {
		t.Fatalf("tick 9 sent %d POSTs, want 5", got)
	}
	for j := 0; j < 5; j++ {
		want := 4 + j
		sets := bodySets(t, h.stub.Body(before+j))
		if !sameInts(sets, []int{want}) {
			t.Fatalf("tick 9 POST %d carries sets %v, want [%d]", j, sets, want)
		}
		if sha, ok := firstSha[want]; ok && sha != h.stub.Sha(before+j) {
			t.Fatalf("set %d resend is not byte-identical to its first POST", want)
		}
	}
}

func TestTelemetryTransport_T5_MaxAge(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.SetDefault(stubStep{status: 503})
	for k := 1; k <= 3; k++ {
		h.record(k)
		h.advance(60 * time.Second)
	}
	h.advance(6 * 60 * time.Second)
	if got := h.state().RetainedCount; got != 0 {
		t.Fatalf("retained_count = %d, want 0 (older than 5 min)", got)
	}
	h.stub.SetDefault(stubStep{status: 200})
	before := h.stub.PostCount()
	h.advance(60 * time.Second)
	if got := h.stub.PostCount(); got != before {
		t.Fatalf("discarded batches were POSTed: %d new POSTs", got-before)
	}
}

func TestTelemetryTransport_T5_OversizeBatchDropped(t *testing.T) {
	h := newTransportHarness(t, WithTelemetryMaxRetainedBytes(4096))
	h.stub.SetDefault(stubStep{status: 503})
	for i := 0; i < transportFixtureConfigs; i++ {
		ctx := NewContextSet().WithNamedContextValues("user", map[string]interface{}{
			"key": fmt.Sprintf("big-%d", i),
			"bio": strings.Repeat("x", 100),
		})
		if _, _, err := h.client.GetStringValue(fmt.Sprintf("cfg-%02d", i), ctx); err != nil {
			t.Fatal(err)
		}
	}
	h.waitRecorded()
	h.advance(60 * time.Second)
	if got := h.stub.PostCount(); got != 1 {
		t.Fatalf("post_count = %d, want 1", got)
	}
	if n := len(h.stub.Body(0)); n <= 4096 {
		t.Fatalf("fixture batch is %d bytes, not oversize", n)
	}
	st := h.state()
	if st.RetainedCount != 0 || st.RetainedBytes != 0 {
		t.Fatalf("oversize batch retained: %d / %d bytes", st.RetainedCount, st.RetainedBytes)
	}
	if got := h.logs.LogCount(lvWarn, ".*"); got != 1 {
		t.Fatalf("WARN count = %d, want 1:\n%s", got, h.logs.Dump())
	}
}

func TestTelemetryTransport_T5_ShippedQueueDefaults(t *testing.T) {
	o := defaultOptions()
	if o.TelemetryMaxRetainedBatches != 5 || o.TelemetryMaxRetainedBytes != 2097152 || o.TelemetryMaxRetainedAge != 300000*time.Millisecond {
		t.Fatalf("queue defaults = %d / %d / %v, want 5 / 2097152 / 5m",
			o.TelemetryMaxRetainedBatches, o.TelemetryMaxRetainedBytes, o.TelemetryMaxRetainedAge)
	}
	if o.TelemetryMaxEvaluationSummaries != 10000 || o.TelemetryMaxContextShapeFields != 10000 || o.TelemetryMaxExampleContexts != 10000 {
		t.Fatalf("aggregator defaults = %d / %d / %d, want 10000 each",
			o.TelemetryMaxEvaluationSummaries, o.TelemetryMaxContextShapeFields, o.TelemetryMaxExampleContexts)
	}
	h := newTransportHarness(t)
	cfg := h.sub.ResolvedConfig()
	if cfg.MaxRetainedBatches != 5 || cfg.MaxRetainedBytes != 2097152 || cfg.MaxRetainedAge != 5*time.Minute ||
		cfg.MaxEvaluationSummaries != 10000 || cfg.MaxContextShapeFields != 10000 || cfg.MaxExampleContexts != 10000 {
		t.Fatalf("resolved caps = %+v", cfg)
	}
}

func TestTelemetryTransport_T5_AggregatorCaps(t *testing.T) {
	t.Run("evaluation summaries", func(t *testing.T) {
		h := newTransportHarness(t, WithTelemetryMaxEvaluationSummaries(3), WithContextTelemetryMode(ContextTelemetryNone))
		for i := 0; i < 5; i++ {
			_, _, _ = h.client.GetStringValue(fmt.Sprintf("cfg-%02d", i), nil)
		}
		// Existing key after the cap still counts.
		_, _, _ = h.client.GetStringValue("cfg-00", nil)
		h.waitRecorded()
		h.advance(60 * time.Second)
		var payload struct {
			Events []struct {
				Summaries *struct {
					Summaries []struct {
						Key      string `json:"key"`
						Counters []struct {
							Count int64 `json:"count"`
						} `json:"counters"`
					} `json:"summaries"`
				} `json:"summaries"`
			} `json:"events"`
		}
		if err := json.Unmarshal(h.stub.Body(0), &payload); err != nil {
			t.Fatal(err)
		}
		counts := map[string]int64{}
		for _, ev := range payload.Events {
			if ev.Summaries == nil {
				continue
			}
			for _, s := range ev.Summaries.Summaries {
				for _, c := range s.Counters {
					counts[s.Key] += c.Count
				}
			}
		}
		if len(counts) != 3 {
			t.Fatalf("summaries carry %d keys %v, want 3", len(counts), counts)
		}
		if counts["cfg-00"] != 2 {
			t.Fatalf("cfg-00 count = %d, want 2 (existing key keeps counting at the cap)", counts["cfg-00"])
		}
		if _, ok := counts["cfg-04"]; ok {
			t.Fatal("key beyond the cap was recorded")
		}
	})

	t.Run("context shape fields", func(t *testing.T) {
		h := newTransportHarness(t, WithTelemetryMaxContextShapeFields(3), WithCollectEvaluationSummaries(false))
		ctx := NewContextSet().WithNamedContextValues("user", map[string]interface{}{
			"key": "u1", "f1": "a", "f2": "b", "f3": "c", "f4": "d",
		})
		_, _, _ = h.client.GetStringValue("cfg-00", ctx)
		h.waitRecorded()
		h.advance(60 * time.Second)
		var payload struct {
			Events []struct {
				ContextShapes *struct {
					Shapes []struct {
						FieldTypes map[string]int `json:"fieldTypes"`
					} `json:"shapes"`
				} `json:"contextShapes"`
			} `json:"events"`
		}
		if err := json.Unmarshal(h.stub.Body(0), &payload); err != nil {
			t.Fatal(err)
		}
		fields := 0
		for _, ev := range payload.Events {
			if ev.ContextShapes == nil {
				continue
			}
			for _, s := range ev.ContextShapes.Shapes {
				fields += len(s.FieldTypes)
			}
		}
		if fields != 3 {
			t.Fatalf("context shapes carry %d fields, want 3; body %s", fields, h.stub.Body(0))
		}
	})

	t.Run("example contexts", func(t *testing.T) {
		h := newTransportHarness(t, WithTelemetryMaxExampleContexts(3), WithCollectEvaluationSummaries(false))
		for i := 0; i < 5; i++ {
			ctx := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"key": fmt.Sprintf("u%d", i)})
			_, _, _ = h.client.GetStringValue("cfg-00", ctx)
		}
		h.waitRecorded()
		h.advance(60 * time.Second)
		var payload struct {
			Events []struct {
				ExampleContexts *struct {
					Examples []json.RawMessage `json:"examples"`
				} `json:"exampleContexts"`
			} `json:"events"`
		}
		if err := json.Unmarshal(h.stub.Body(0), &payload); err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, ev := range payload.Events {
			if ev.ExampleContexts != nil {
				n += len(ev.ExampleContexts.Examples)
			}
		}
		if n != 3 {
			t.Fatalf("example contexts = %d, want 3", n)
		}
	})
}

// ---------------------------------------------------------------------------
// T6 - Logging episodes (P7)
// ---------------------------------------------------------------------------

func TestTelemetryTransport_T6a_Blip(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.Script(stubStep{status: 503}, stubStep{status: 200})
	h.record(1)
	h.advance(60 * time.Second)
	h.advance(60 * time.Second)
	if got := h.logs.LogCount(lvWarn, ".*"); got != 0 {
		t.Fatalf("WARN = %d, want 0:\n%s", got, h.logs.Dump())
	}
	if got := h.logs.LogCount(lvInfo, "(?i)recover"); got != 1 {
		t.Fatalf("recovery INFO = %d, want 1:\n%s", got, h.logs.Dump())
	}
	if h.logs.LogCount(lvDebug, ".*") < 1 {
		t.Fatal("no DEBUG line for the failed POST")
	}
	if got := h.logs.LogCount(lvError, ".*"); got != 0 {
		t.Fatalf("ERROR = %d, want 0", got)
	}
}

func TestTelemetryTransport_T6b_SustainedThenRecovery(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.SetDefault(stubStep{status: 503})
	for k := 1; k <= 5; k++ {
		h.record(k)
		h.advance(60 * time.Second)
	}
	if got := h.logs.LogCount(lvWarn, ".*"); got != 0 {
		t.Fatalf("WARN after ticks 1-5 = %d, want 0:\n%s", got, h.logs.Dump())
	}
	h.record(6)
	h.advance(60 * time.Second) // tick 6: first drop (t=360s)
	if got := h.logs.LogCount(lvWarn, ".*"); got != 1 {
		t.Fatalf("WARN after tick 6 = %d, want 1:\n%s", got, h.logs.Dump())
	}
	if got := h.logs.LogCount(lvWarn, `503.*1 batch\(es\) dropped.*retained queue 5/5`); got != 1 {
		t.Fatalf("first-drop WARN lacks status / dropped count / depth:\n%s", h.logs.Dump())
	}
	for k := 7; k <= 12; k++ { // drops within 10 min of the WARN
		h.record(k)
		h.advance(60 * time.Second)
		if got := h.logs.LogCount(lvWarn, ".*"); got != 1 {
			t.Fatalf("WARN after tick %d = %d, want 1:\n%s", k, got, h.logs.Dump())
		}
	}
	h.stub.SetDefault(stubStep{status: 200})
	h.advance(60 * time.Second) // tick 13 < 10 min after the WARN
	if got := h.logs.LogCount(lvInfo, "(?i)recover"); got != 1 {
		t.Fatalf("recovery INFO = %d, want 1:\n%s", got, h.logs.Dump())
	}
	if got := h.logs.LogCount(lvError, ".*"); got != 0 {
		t.Fatalf("ERROR = %d, want 0", got)
	}
}

func TestTelemetryTransport_T6c_WarnSummaryCadence(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.SetDefault(stubStep{status: 503})
	for k := 1; k <= 6; k++ { // first WARN at tick 6 (t=360s)
		h.record(k)
		h.advance(60 * time.Second)
	}
	if got := h.logs.LogCount(lvWarn, ".*"); got != 1 {
		t.Fatalf("WARN after tick 6 = %d, want 1:\n%s", got, h.logs.Dump())
	}
	for k := 7; k <= 15; k++ { // t=420..900s: within 10 min
		h.record(k)
		h.advance(60 * time.Second)
	}
	if got := h.logs.LogCount(lvWarn, ".*"); got != 1 {
		t.Fatalf("WARN before 10 min = %d, want 1:\n%s", got, h.logs.Dump())
	}
	h.record(16)
	h.advance(60 * time.Second) // t=960s: exactly 10 min after the first WARN
	if got := h.logs.LogCount(lvWarn, ".*"); got != 2 {
		t.Fatalf("WARN at 10 min = %d, want 2:\n%s", got, h.logs.Dump())
	}
	if got := h.logs.LogCount(lvWarn, `(?i)still dropping.*10 batch\(es\) dropped`); got != 1 {
		t.Fatalf("summary WARN lacks the drop count since the last WARN:\n%s", h.logs.Dump())
	}
	for k := 17; k <= 25; k++ { // < another 10 min
		h.record(k)
		h.advance(60 * time.Second)
	}
	if got := h.logs.LogCount(lvWarn, ".*"); got != 2 {
		t.Fatalf("WARN after another 9 min = %d, want 2:\n%s", got, h.logs.Dump())
	}
	if got := h.logs.LogCount(lvError, ".*"); got != 0 {
		t.Fatalf("ERROR = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// T7 - One POST in flight (P2)
// ---------------------------------------------------------------------------

func TestTelemetryTransport_T7_OnePostInFlight(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.Script(stubStep{hang: true})
	h.record(1) // A
	h.advance(60 * time.Second)
	h.waitPosts(1)

	h.record(2) // B
	h.sub.Tick()
	h.record(3) // C
	h.sub.Tick()
	if got := h.stub.PostCount(); got != 1 {
		t.Fatalf("post_count = %d, want 1 (ticks while busy must skip)", got)
	}

	h.stub.Release(0, stubStep{status: 200})
	h.waitIdle()
	h.sub.Tick()
	if got := h.stub.PostCount(); got != 2 {
		t.Fatalf("post_count = %d, want 2", got)
	}
	if got := bodySets(t, h.stub.Body(1)); !sameInts(got, []int{2, 3}) {
		t.Fatalf("body(1) sets = %v, want [2 3]", got)
	}
}

// ---------------------------------------------------------------------------
// T8 - Shutdown (P8)
// ---------------------------------------------------------------------------

func TestTelemetryTransport_T8_Shutdown(t *testing.T) {
	h := newTransportHarness(t)
	h.stub.Script(stubStep{status: 503}, stubStep{status: 503})
	h.record(1)
	h.advance(60 * time.Second)
	h.record(2)
	h.advance(60 * time.Second)
	if got := h.state().RetainedCount; got != 2 {
		t.Fatalf("retained_count = %d, want 2", got)
	}
	retained := map[[32]byte]bool{h.stub.Sha(0): true, h.stub.Sha(1): true}
	before := h.stub.PostCount()

	h.record(3)
	h.stub.SetDefault(stubStep{hang: true})

	done := make(chan struct{})
	go func() {
		h.client.Close()
		close(done)
	}()
	h.closed = true
	h.waitPosts(before + 1)
	h.advance(5 * time.Second)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close() did not return within the 5s (virtual) deadline")
	}

	if got := h.stub.PostCount(); got != before+1 {
		t.Fatalf("Close() sent %d POSTs, want exactly 1 (the live window)", got-before)
	}
	final := h.stub.Body(before)
	if retained[sha256.Sum256(final)] {
		t.Fatal("Close() POSTed a retained batch")
	}
	if got := bodySets(t, final); !sameInts(got, []int{3}) {
		t.Fatalf("final flush sets = %v, want [3]", got)
	}

	h.advance(10 * time.Minute)
	if got := h.stub.PostCount(); got != before+1 {
		t.Fatalf("POSTs after Close(): %d", got-before-1)
	}
	st := h.state()
	if st.TimerActive || st.TickRunning || st.PostInFlight || st.ConsumerRunning {
		t.Fatalf("background work left after Close(): %+v", st)
	}
	h.client.Close() // second call is a no-op
}
