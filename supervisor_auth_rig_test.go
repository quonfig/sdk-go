package quonfig

// Rig for supervisor contract Test 8 (auth-class response, qfg-goi1.10.23;
// integration-test-data chaos/supervisor-test-contract.md at v2026.10.16).
//
// The rig follows the contract's fixture rule: the supervisor, the SSE
// client, the fallback-poller schedule and the auth-failed state machine are
// all real. Only these are faked:
//
//   - the SSE worker's server: an httptest server whose status per connect
//     attempt the test scripts;
//   - the config fetch legs: one httptest server per leg, each a per-leg
//     function returning (status, body) for a given fetch cycle;
//   - the clock: every background wait the SDK requests goes through
//     withTestWaitHook, which records the requested duration and waits
//     1/t8Compression of it, so the test asserts the requested values
//     (anti-vacuity rule) without sleeping minutes.
//
// Everything lands in one ordered event log (waits, server hits, log
// records), so the assertions can cut it at the entry and exit points.

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	t8Key         = "t8-secret-sdk-key"
	t8Compression = 2000
)

var t8AuthStatuses = []int{401, 403, 404, 410}

type t8Event struct {
	kind   string // "wait:<kind>", "hit:sse", "hit:0", "hit:1", "log"
	d      time.Duration
	level  slog.Level
	msg    string
	state  ConnectionState
	active bool
}

// t8Resp is one leg's scripted answer for one fetch cycle.
type t8Resp struct {
	status int           // 200 sends a valid envelope at gen; 304 sends Not Modified
	gen    int           // envelope generation for a 200
	delay  time.Duration // sleep before answering (timeout sub-tests)
	exit   bool          // mark the rig "exited" when served (SSE then answers sseAfter)
}

type t8Rig struct {
	t      *testing.T
	mu     sync.Mutex
	events []t8Event

	client atomic.Pointer[Client]
	exited atomic.Bool

	sseStatus int // SSE status before the exit response is served
	sseAfter  int // SSE status after it (0 = same as sseStatus)
	// leg returns leg's answer for fetch cycle n (n counts primary-leg
	// requests, so cycle 1 is the init fetch).
	leg func(leg, cycle int) t8Resp

	leg0Calls atomic.Int32
	sseCalls  atomic.Int32
}

func (r *t8Rig) record(e t8Event) {
	r.mu.Lock()
	r.events = append(r.events, e)
	r.mu.Unlock()
}

func (r *t8Rig) snapshot() []t8Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]t8Event(nil), r.events...)
}

func (r *t8Rig) wait(kind string, d time.Duration) time.Duration {
	r.record(t8Event{kind: "wait:" + kind, d: d})
	return d / t8Compression
}

func (r *t8Rig) clientState() (ConnectionState, bool) {
	c := r.client.Load()
	if c == nil {
		return "", false
	}
	return c.ConnectionState(), c.FallbackPollerActive()
}

func (r *t8Rig) sseHandler(w http.ResponseWriter, _ *http.Request) {
	r.sseCalls.Add(1)
	st, active := r.clientState()
	r.record(t8Event{kind: "hit:sse", state: st, active: active})
	status := r.sseStatus
	if r.exited.Load() && r.sseAfter != 0 {
		status = r.sseAfter
	}
	http.Error(w, http.StatusText(status), status)
}

func (r *t8Rig) legHandler(leg int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		var cycle int
		if leg == 0 {
			cycle = int(r.leg0Calls.Add(1))
		} else {
			cycle = int(r.leg0Calls.Load())
		}
		st, active := r.clientState()
		r.record(t8Event{kind: fmt.Sprintf("hit:%d", leg), state: st, active: active})
		resp := r.leg(leg, cycle)
		if resp.delay > 0 {
			time.Sleep(resp.delay)
		}
		if resp.exit {
			r.exited.Store(true)
		}
		switch resp.status {
		case http.StatusOK:
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"configs":[],"meta":{"version":"gen-%d","environment":"Production","generation":%d}}`, resp.gen, resp.gen)
		case http.StatusNotModified:
			w.WriteHeader(http.StatusNotModified)
		default:
			http.Error(w, http.StatusText(resp.status), resp.status)
		}
	}
}

// t8LogHandler appends every record (with its attrs) to the rig's event log.
type t8LogHandler struct{ r *t8Rig }

func (h t8LogHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h t8LogHandler) Handle(_ context.Context, rec slog.Record) error {
	var b strings.Builder
	b.WriteString(rec.Message)
	rec.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%v", a.Key, a.Value)
		return true
	})
	h.r.record(t8Event{kind: "log", level: rec.Level, msg: b.String()})
	return nil
}
func (h t8LogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h t8LogHandler) WithGroup(string) slog.Handler      { return h }

// start builds the servers and the client. legs is 1 or 2; extra options
// (polling mode, hedge abort) are appended.
func (r *t8Rig) start(legs int, extra ...Option) *Client {
	t := r.t
	sse := httptest.NewServer(http.HandlerFunc(r.sseHandler))
	t.Cleanup(sse.Close)
	var urls []string
	for i := 0; i < legs; i++ {
		srv := httptest.NewServer(r.legHandler(i))
		t.Cleanup(srv.Close)
		urls = append(urls, srv.URL)
	}
	opts := []Option{
		WithSdkKey(t8Key),
		WithAPIURLs(urls),
		WithAllTelemetryDisabled(),
		WithInitTimeout(8 * time.Second),
		WithLogger(slog.New(t8LogHandler{r: r})),
		withTestStreamURLOverride(sse.URL),
		withTestWaitHook(r.wait),
	}
	opts = append(opts, extra...)
	c, err := NewClient(opts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	r.client.Store(c)
	t.Cleanup(c.Close)
	return c
}

// await polls cond for up to 10s of wall clock.
func (r *t8Rig) await(what string, cond func([]t8Event) bool) []t8Event {
	r.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		ev := r.snapshot()
		if cond(ev) {
			return ev
		}
		time.Sleep(2 * time.Millisecond)
	}
	ev := r.snapshot()
	r.t.Fatalf("timed out waiting for %s; events:\n%s", what, t8Dump(ev))
	return nil
}

func t8Count(ev []t8Event, kind string) int {
	n := 0
	for _, e := range ev {
		if e.kind == kind {
			n++
		}
	}
	return n
}

// t8Index returns the index of the nth (1-based) event of kind at or after
// from, or -1.
func t8Index(ev []t8Event, from int, kind string, nth int) int {
	for i := from; i < len(ev); i++ {
		if ev[i].kind == kind {
			nth--
			if nth == 0 {
				return i
			}
		}
	}
	return -1
}

func t8Logs(ev []t8Event, level slog.Level) []string {
	var out []string
	for _, e := range ev {
		if e.kind == "log" && e.level == level {
			out = append(out, e.msg)
		}
	}
	return out
}

func t8Waits(ev []t8Event, kind string) []time.Duration {
	var out []time.Duration
	for _, e := range ev {
		if e.kind == "wait:"+kind {
			out = append(out, e.d)
		}
	}
	return out
}

// t8CheckSSELadder asserts the SSE reconnect sleeps follow the supervisor
// Test 2 ladder from a fresh start: the kth sleep is the jittered value of
// base_k = min(500ms * 2^k, 30s), i.e. within [base_k/2, base_k].
func t8CheckSSELadder(t *testing.T, waits []time.Duration, label string) {
	t.Helper()
	base := 500 * time.Millisecond
	for k, w := range waits {
		if w < base/2 || w > base {
			t.Errorf("%s: SSE sleep #%d = %s, want within [%s, %s] (normal ladder)", label, k+1, w, base/2, base)
		}
		base *= 2
		if base > 30*time.Second {
			base = 30 * time.Second
		}
	}
}

func t8Dump(ev []t8Event) string {
	var b strings.Builder
	for i, e := range ev {
		switch e.kind {
		case "log":
			fmt.Fprintf(&b, "%4d log %s %s\n", i, e.level, e.msg)
		default:
			if strings.HasPrefix(e.kind, "wait:") {
				fmt.Fprintf(&b, "%4d %s %s\n", i, e.kind, e.d)
			} else {
				fmt.Fprintf(&b, "%4d %s state=%s active=%v\n", i, e.kind, e.state, e.active)
			}
		}
	}
	return b.String()
}
