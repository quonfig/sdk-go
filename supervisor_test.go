package quonfig

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Tier 1 supervisor unit tests per qfg-47c2.17 / B3 contract.
//
// The Supervisor abstraction owns one or more long-running workers and:
//   - recovers from panic and error returns at the worker boundary,
//   - restarts the worker with exponential backoff (500ms → 30s cap),
//   - bumps quonfig_sdk_worker_restart_total{layer="<n>"} per restart,
//   - stops cleanly within 5s on Stop(),
//   - exposes ConnectionState() for callers. (LastSuccessfulRefresh lives on
//     the Client since qfg-41nh.11 — see refresh_liveness_internal_test.go
//     and TestClientConnectionStateAndLastRefreshTransitions.)
//
// These tests inject tiny (sub-millisecond) backoff bounds so the suite
// completes in well under a second; the actual 500ms→30s exponential
// formula is verified separately in TestSupervisorExponentialBackoffFormula.

// Test 1 — Supervisor restarts a worker that panics within 1000ms.
func TestSupervisorRestartsPanickedWorker(t *testing.T) {
	var calls atomic.Int32
	restarted := make(chan struct{})

	w := worker{
		Layer: "1",
		Run: func(ctx context.Context) error {
			n := calls.Add(1)
			if n == 1 {
				// First invocation panics; the supervisor must catch it,
				// restart us, and we'll close `restarted` from the second
				// invocation as proof of life.
				panic("boom")
			}
			close(restarted)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	s := newSupervisor(supervisorConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	}, w)
	s.Start()
	defer s.Stop()

	select {
	case <-restarted:
	case <-time.After(1 * time.Second):
		t.Fatalf("supervisor did not restart panicked worker within 1s; calls=%d", calls.Load())
	}
}

// Test 2 — Exponential backoff (500ms → 1s → 2s → ... → 30s cap).
//
// We verify the formula directly. The Start/Stop loop uses time.After so
// asserting the cap in a real-time integration test would take ~62s. The
// formula is the only invariant worth pinning here.
func TestSupervisorExponentialBackoffFormula(t *testing.T) {
	s := newSupervisor(supervisorConfig{
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     30 * time.Second,
	})
	want := []time.Duration{
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second, // 32s would exceed cap → 30s
		30 * time.Second, // cap holds
		30 * time.Second,
	}
	for i, w := range want {
		if got := s.backoffFor(i); got != w {
			t.Errorf("backoffFor(%d) = %s, want %s", i, got, w)
		}
	}
}

// Test 3 — Clean shutdown within 5s on Stop().
func TestSupervisorStopJoinsWithinDeadline(t *testing.T) {
	var running atomic.Bool
	w := worker{
		Layer: "1",
		Run: func(ctx context.Context) error {
			running.Store(true)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	s := newSupervisor(supervisorConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	}, w)
	s.Start()

	// Wait until the worker is observably running so Stop is exercised
	// against a live goroutine, not an unstarted one.
	deadline := time.Now().Add(500 * time.Millisecond)
	for !running.Load() && time.Now().Before(deadline) {
		time.Sleep(1 * time.Millisecond)
	}
	if !running.Load() {
		t.Fatal("worker never started")
	}

	stopStart := time.Now()
	stopDone := make(chan struct{})
	go func() {
		s.Stop()
		close(stopDone)
	}()

	select {
	case <-stopDone:
		if elapsed := time.Since(stopStart); elapsed > 5*time.Second {
			t.Errorf("Stop took %s, want ≤5s", elapsed)
		}
	case <-time.After(5500 * time.Millisecond):
		t.Fatal("Stop() did not return within 5.5s")
	}
}

// Test 4 — worker_restart_total{layer="1"} increments per restart.
func TestSupervisorWorkerRestartTotalIncrements(t *testing.T) {
	var calls atomic.Int32
	const wantRestarts = 3

	done := make(chan struct{})
	w := worker{
		Layer: "1",
		Run: func(ctx context.Context) error {
			n := calls.Add(1)
			if int(n) <= wantRestarts {
				panic("boom")
			}
			close(done)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	s := newSupervisor(supervisorConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	}, w)
	s.Start()
	defer s.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("worker never reached steady state; calls=%d", calls.Load())
	}

	if got := s.WorkerRestartTotal("1"); got != wantRestarts {
		t.Errorf("WorkerRestartTotal(\"1\") = %d, want %d", got, wantRestarts)
	}
	if got := s.WorkerRestartTotal("2"); got != 0 {
		t.Errorf("WorkerRestartTotal(\"2\") = %d, want 0 (untouched layer)", got)
	}
}

// Supervisor last line of defense — a panic that surfaces *from* the worker
// (e.g. a panic that the in-worker OnEnvelope recover() failed to catch, or a
// future worker shape that re-raises) is caught by the supervisor. That IS a
// worker restart. Contract Test 5 (a user callback throw is caught in place
// and is not a restart) is in callback_errors_test.go.
// The supervisor restarts the worker, the process stays alive, and the
// restart counter records the event under layer="1".
func TestSupervisorRecoversFromOnEnvelopeStylePanic(t *testing.T) {
	var phase atomic.Int32 // 0 = pre-panic, 1 = post-restart
	resumed := make(chan struct{})

	w := worker{
		Layer: "1",
		Run: func(ctx context.Context) error {
			if phase.Load() == 0 {
				phase.Store(1)
				// Simulate the worst case: the OnEnvelope callback panics and
				// the in-worker invokeOnEnvelope guard is absent or bypassed.
				// The supervisor must catch it.
				panic("user OnEnvelope handler exploded")
			}
			close(resumed)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	s := newSupervisor(supervisorConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	}, w)
	s.Start()
	defer s.Stop()

	select {
	case <-resumed:
	case <-time.After(1 * time.Second):
		t.Fatalf("supervisor did not recover from OnEnvelope-style panic; phase=%d", phase.Load())
	}
	if got := s.WorkerRestartTotal("1"); got < 1 {
		t.Errorf("WorkerRestartTotal(\"1\") = %d, want ≥1", got)
	}
}

// Test 6 — ConnectionState transitions through documented values as the
// worker reports state changes. The supervisor itself does not own the
// transport — it provides the surface that workers report into. (The
// LastSuccessfulRefresh half of contract Test 6 is covered at the Client
// level, where the stamp lives since qfg-41nh.11.)
func TestSupervisorConnectionState(t *testing.T) {
	gate := make(chan struct{})
	w := worker{
		Layer: "1",
		Run: func(ctx context.Context) error {
			// Worker contract: report state transitions through the supervisor.
			// The supervisor is the source of truth for ConnectionState().
			<-gate
			<-ctx.Done()
			return ctx.Err()
		},
	}

	s := newSupervisor(supervisorConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	}, w)

	// Before Start: initializing.
	if got, want := s.ConnectionState(), ConnStateInitializing; got != want {
		t.Errorf("pre-Start ConnectionState = %q, want %q", got, want)
	}

	s.Start()
	defer s.Stop()

	// Drive state transitions the way a worker would.
	s.setConnectionState(ConnStateConnected)
	if got, want := s.ConnectionState(), ConnStateConnected; got != want {
		t.Errorf("after connected ConnectionState = %q, want %q", got, want)
	}

	s.setConnectionState(ConnStateDisconnected)
	if got, want := s.ConnectionState(), ConnStateDisconnected; got != want {
		t.Errorf("after disconnect ConnectionState = %q, want %q", got, want)
	}

	s.setConnectionState(ConnStateFallingBack)
	if got, want := s.ConnectionState(), ConnStateFallingBack; got != want {
		t.Errorf("after fallback ConnectionState = %q, want %q", got, want)
	}

	close(gate)
}

// Sanity check — clean exit (worker returns context error after cancel) does
// not count as a restart. Catches an off-by-one where Stop()'s cancellation
// inflates worker_restart_total.
func TestSupervisorCleanShutdownDoesNotCountAsRestart(t *testing.T) {
	w := worker{
		Layer: "1",
		Run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	s := newSupervisor(supervisorConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	}, w)
	s.Start()
	time.Sleep(20 * time.Millisecond)
	s.Stop()

	if got := s.WorkerRestartTotal("1"); got != 0 {
		t.Errorf("clean shutdown WorkerRestartTotal(\"1\") = %d, want 0", got)
	}
}

// Sanity check — a worker that returns a non-context error is counted as a
// restart. Tier 1 test 4 names "panic"; this nails down the "any unexpected
// exit" semantic the supervisor must implement to be useful.
func TestSupervisorErrorReturnCountsAsRestart(t *testing.T) {
	var calls atomic.Int32
	done := make(chan struct{})

	w := worker{
		Layer: "1",
		Run: func(ctx context.Context) error {
			n := calls.Add(1)
			if n == 1 {
				return errors.New("transient failure")
			}
			close(done)
			<-ctx.Done()
			return ctx.Err()
		},
	}

	s := newSupervisor(supervisorConfig{
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	}, w)
	s.Start()
	defer s.Stop()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("supervisor did not restart errored worker; calls=%d", calls.Load())
	}
	if got := s.WorkerRestartTotal("1"); got < 1 {
		t.Errorf("WorkerRestartTotal(\"1\") = %d, want ≥1", got)
	}
}

// ---------------------------------------------------------------------------
// Test 8 — Auth-class response (contract v2026.10.16, plan
// project/plans/2026-10-07-sdk-auth-failure-rule.md R1-R6, qfg-goi1.10.23).
// Rig: supervisor_auth_rig_test.go. Every sub-test runs over 401/403/404/410.
// ---------------------------------------------------------------------------

// t8EnteringCycle returns the index of the entering cycle's primary hit: the
// first primary-leg request after SSE started (cycle 1 is the init fetch,
// which runs before SSE).
func t8EnteringCycle(t *testing.T, ev []t8Event) int {
	t.Helper()
	firstSSE := t8Index(ev, 0, "hit:sse", 1)
	if firstSSE < 0 {
		t.Fatalf("SSE never attempted")
	}
	i := t8Index(ev, firstSSE, "hit:0", 1)
	if i < 0 {
		t.Fatalf("no fetch cycle after SSE started")
	}
	return i
}

// Test 8 (a)-(c): SSE gets auth-class, then the fallback poller's cycle gets
// auth-class on every leg -> auth-failed: 6 probes each requested at a wait in
// [150s, 300s], SSE and the poller off, disconnected, one ERROR, no WARN.
func TestSupervisor_AuthClassEntersAuthFailedAndProbesSlowly(t *testing.T) {
	for _, status := range t8AuthStatuses {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			r := &t8Rig{t: t, sseStatus: status}
			r.leg = func(leg, cycle int) t8Resp {
				if cycle == 1 {
					return t8Resp{status: 200, gen: 1}
				}
				return t8Resp{status: status}
			}
			r.start(2)
			// Entering cycle (2) + 6 probes (3..8), then the 7th probe wait.
			ev := r.await("6 probes", func(ev []t8Event) bool {
				return r.leg0Calls.Load() >= 8 && t8Count(ev, "wait:probe") >= 7
			})

			enter := t8EnteringCycle(t, ev)
			pre := ev[:enter]
			// (a) before the entering cycle: SSE on its normal ladder, the
			// poller engaged on its existing 120s rule, no other fetch cycle.
			if n := t8Count(pre, "hit:sse"); n < 3 {
				t.Errorf("only %d SSE attempts before the entering cycle", n)
			}
			t8CheckSSELadder(t, t8Waits(pre, "sse"), "pre-entry")
			if got := t8Waits(pre, "engage"); len(got) != 1 || got[0] != DefaultFallbackPollThreshold {
				t.Errorf("engage waits before entry = %v, want [%s]", got, DefaultFallbackPollThreshold)
			}
			if n := t8Count(ev[t8Index(ev, 0, "hit:sse", 1):enter], "hit:1"); n != 0 {
				t.Errorf("%d fetch requests between SSE start and the entering cycle, want 0", n)
			}

			errIdx := -1
			for i := enter; i < len(ev); i++ {
				if ev[i].kind == "log" && ev[i].level == slog.LevelError {
					errIdx = i
					break
				}
			}
			if errIdx < 0 {
				t.Fatalf("no ERROR logged after the entering cycle; events:\n%s", t8Dump(ev))
			}
			post := ev[errIdx:]

			// (a) the next 6 probes: requested at waits in [150s, 300s].
			probes := t8Waits(post, "probe")
			if len(probes) < 7 {
				t.Fatalf("probe waits after entry = %v, want >= 7", probes)
			}
			for i, w := range probes[:7] {
				if w < 150*time.Second || w > 300*time.Second {
					t.Errorf("probe wait #%d = %s, want within [150s, 300s]", i+1, w)
				}
			}

			// (b) while auth-failed: no SSE attempt, the poller never
			// engages or ticks, each probe is one cycle across both legs,
			// disconnected throughout.
			last := t8Index(ev, errIdx, "wait:probe", 7)
			window := ev[errIdx:last]
			for _, k := range []string{"hit:sse", "wait:sse", "wait:engage", "wait:poll"} {
				if n := t8Count(window, k); n != 0 {
					t.Errorf("%d %s events while auth-failed, want 0", n, k)
				}
			}
			for p := 1; p <= 6; p++ {
				a := t8Index(ev, errIdx, "wait:probe", p)
				b := t8Index(ev, errIdx, "wait:probe", p+1)
				seg := ev[a:b]
				if n0, n1 := t8Count(seg, "hit:0"), t8Count(seg, "hit:1"); n0 != 1 || n1 != 1 {
					t.Errorf("probe %d asked primary %d and secondary %d times, want 1 and 1", p, n0, n1)
				}
			}
			for _, e := range window {
				if e.kind == "hit:0" || e.kind == "hit:1" {
					if e.state != ConnStateDisconnected || e.active {
						t.Errorf("during a probe: ConnectionState=%q FallbackPollerActive=%v, want disconnected/false", e.state, e.active)
					}
				}
			}

			// (c) from entry through the 6 probes: one ERROR naming the
			// status (never the key), no other ERROR, no WARN.
			counted := ev[enter:last]
			errs := t8Logs(counted, slog.LevelError)
			if len(errs) != 1 || !strings.Contains(errs[0], fmt.Sprint(status)) {
				t.Errorf("ERROR lines from entry = %q, want exactly one naming %d", errs, status)
			}
			if warns := t8Logs(counted, slog.LevelWarn); len(warns) != 0 {
				t.Errorf("WARN lines from entry = %q, want none", warns)
			}
			for _, e := range ev {
				if e.kind == "log" && strings.Contains(e.msg, t8Key) {
					t.Errorf("log line contains the SDK key: %s", e.msg)
				}
			}
		})
	}
}

// Test 8 (d): a probe that gets a valid 200 on the secondary leg only, or a
// 304 on any leg, exits: install, one INFO, SSE restarts from its base
// backoff, the poller returns to its normal schedule.
func TestSupervisor_AuthFailedExitsOnFirstValidAnswer(t *testing.T) {
	type exitCase struct {
		name            string
		primary, second t8Resp
		wantGen         int
	}
	for _, status := range t8AuthStatuses {
		cases := []exitCase{
			{"secondary-200", t8Resp{status: status}, t8Resp{status: 200, gen: 2, exit: true}, 2},
			{"primary-304", t8Resp{status: 304, exit: true}, t8Resp{status: status}, 1},
			{"secondary-304", t8Resp{status: status}, t8Resp{status: 304, exit: true}, 1},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%d/%s", status, tc.name), func(t *testing.T) {
				r := &t8Rig{t: t, sseStatus: status, sseAfter: 503}
				// cycle 2 enters, cycle 3 is a rejected probe, cycle 4 exits.
				r.leg = func(leg, cycle int) t8Resp {
					switch {
					case cycle == 1:
						return t8Resp{status: 200, gen: 1}
					case cycle == 4 && leg == 0:
						return tc.primary
					case cycle == 4:
						return tc.second
					}
					return t8Resp{status: status}
				}
				c := r.start(2)
				ev := r.await("exit and an SSE restart", func(ev []t8Event) bool {
					i := t8Index(ev, 0, "wait:probe", 2)
					if !r.exited.Load() || i < 0 {
						return false
					}
					h := t8Index(ev, i, "hit:sse", 1)
					return h >= 0 && t8Index(ev, h, "wait:sse", 1) >= 0 && t8Index(ev, i, "wait:engage", 1) >= 0
				})
				if got := c.HeldGeneration(); got != tc.wantGen {
					t.Errorf("HeldGeneration = %d, want %d", got, tc.wantGen)
				}
				enter := t8EnteringCycle(t, ev)
				errIdx := t8Index(ev, enter, "log", 1)
				for errIdx >= 0 && ev[errIdx].level != slog.LevelError {
					errIdx = t8Index(ev, errIdx+1, "log", 1)
				}
				exitIdx := -1
				for i := errIdx; errIdx >= 0 && i < len(ev); i++ {
					if ev[i].kind == "log" && ev[i].level == slog.LevelInfo {
						exitIdx = i
						break
					}
				}
				if errIdx < 0 || exitIdx < 0 {
					t.Fatalf("want an entry ERROR then an exit INFO; events:\n%s", t8Dump(ev))
				}
				restart := t8Index(ev, exitIdx, "hit:sse", 1)
				if n := t8Count(ev[errIdx:exitIdx], "hit:sse"); n != 0 {
					t.Errorf("%d SSE attempts while auth-failed, want 0", n)
				}
				if n := t8Count(ev[errIdx:exitIdx], "wait:probe"); n != 2 {
					t.Errorf("%d probe waits while auth-failed, want 2 (cycles 3 and 4)", n)
				}
				infos := t8Logs(ev[enter:restart], slog.LevelInfo)
				if len(infos) != 1 {
					t.Errorf("INFO lines from entry to the SSE restart = %q, want exactly one", infos)
				}
				if errs := t8Logs(ev[enter:restart], slog.LevelError); len(errs) != 1 {
					t.Errorf("ERROR lines from entry to exit = %q, want exactly one", errs)
				}
				lastProbe := exitIdx
				// SSE restarts from its normal base: the next ordinary SSE
				// failure requests the Test 2 first backoff.
				afterExit := t8Waits(ev[restart:], "sse")
				if len(afterExit) == 0 {
					t.Fatalf("no SSE sleep after the restart")
				}
				t8CheckSSELadder(t, afterExit[:1], "post-exit")
				// The poller is back on its normal rule (engage after 120s).
				if got := t8Waits(ev[lastProbe:], "engage"); len(got) != 1 || got[0] != DefaultFallbackPollThreshold {
					t.Errorf("engage waits after exit = %v, want [%s]", got, DefaultFallbackPollThreshold)
				}
				if n := t8Count(ev[lastProbe:], "wait:probe"); n != 0 {
					t.Errorf("%d probe waits after exit, want 0", n)
				}
				if st := c.ConnectionState(); st == ConnStateFallingBack && !c.FallbackPollerActive() {
					t.Errorf("ConnectionState = %q with the poller idle", st)
				}
			})
		}
	}
}

// t8CheckNormalRegime asserts a run never entered auth-failed: no ERROR, no
// probe, the SSE ladder continues across the poller's cycles, and the poller
// keeps its normal interval (at least two cycles after engaging).
func t8CheckNormalRegime(t *testing.T, r *t8Rig) {
	t.Helper()
	ev := r.await("3 poller cycles", func(ev []t8Event) bool {
		return r.leg0Calls.Load() >= 4
	})
	if errs := t8Logs(ev, slog.LevelError); len(errs) != 0 {
		t.Errorf("ERROR lines = %q, want none", errs)
	}
	if n := t8Count(ev, "wait:probe"); n != 0 {
		t.Errorf("%d probe waits, want 0 (never auth-failed)", n)
	}
	t8CheckSSELadder(t, t8Waits(ev, "sse"), "whole run")
	firstPoll := t8EnteringCycle(t, ev)
	if n := t8Count(ev[firstPoll:], "hit:sse"); n == 0 {
		t.Errorf("SSE stopped after the poller's cycle; want the normal ladder to continue")
	}
	if got := t8Waits(ev, "poll"); len(got) != 1 || got[0] != DefaultFallbackPollInterval {
		t.Errorf("poll interval requested = %v, want [%s]", got, DefaultFallbackPollInterval)
	}
	for _, e := range ev[firstPoll:] {
		if e.kind == "hit:0" && e.state != ConnStateFallingBack {
			t.Errorf("ConnectionState during a poll = %q, want falling_back", e.state)
		}
	}
}

// Test 8 (e): SSE auth-class, then a poller cycle with a 200 on any leg: no
// auth-failed, no ERROR, SSE and the poller stay on their normal schedule.
func TestSupervisor_TransientPrimaryRejectionStaysNormal(t *testing.T) {
	for _, status := range t8AuthStatuses {
		for _, name := range []string{"secondary-200", "primary-200"} {
			t.Run(fmt.Sprintf("%d/%s", status, name), func(t *testing.T) {
				r := &t8Rig{t: t, sseStatus: status}
				r.leg = func(leg, cycle int) t8Resp {
					if cycle == 1 || name == "primary-200" || leg == 1 {
						return t8Resp{status: 200, gen: 1}
					}
					return t8Resp{status: status}
				}
				r.start(2)
				t8CheckNormalRegime(t, r)
			})
		}
	}
}

// Test 8 (f): primary auth-class + secondary 503 or timeout enters;
// primary 503 + secondary auth-class does not.
func TestSupervisor_MixedLegs(t *testing.T) {
	for _, status := range t8AuthStatuses {
		for _, second := range []string{"503", "timeout"} {
			t.Run(fmt.Sprintf("%d/primary-auth+secondary-%s", status, second), func(t *testing.T) {
				r := &t8Rig{t: t, sseStatus: status}
				r.leg = func(leg, cycle int) t8Resp {
					switch {
					case cycle == 1:
						return t8Resp{status: 200, gen: 1}
					case leg == 0:
						return t8Resp{status: status}
					case second == "timeout":
						return t8Resp{status: 200, gen: 9, delay: 400 * time.Millisecond}
					}
					return t8Resp{status: 503}
				}
				r.start(2, WithConfigFetchHedgeAbort(100*time.Millisecond))
				ev := r.await("a probe after entry", func(ev []t8Event) bool {
					return t8Count(ev, "wait:probe") >= 2
				})
				errs := t8Logs(ev, slog.LevelError)
				if len(errs) != 1 || !strings.Contains(errs[0], fmt.Sprint(status)) {
					t.Errorf("ERROR lines = %q, want exactly one naming %d", errs, status)
				}
				for i, w := range t8Waits(ev, "probe") {
					if w < 150*time.Second || w > 300*time.Second {
						t.Errorf("probe wait #%d = %s, want within [150s, 300s]", i+1, w)
					}
				}
			})
		}
		t.Run(fmt.Sprintf("%d/primary-503+secondary-auth", status), func(t *testing.T) {
			r := &t8Rig{t: t, sseStatus: status}
			r.leg = func(leg, cycle int) t8Resp {
				switch {
				case cycle == 1:
					return t8Resp{status: 200, gen: 1}
				case leg == 0:
					return t8Resp{status: 503}
				}
				return t8Resp{status: status}
			}
			r.start(2)
			t8CheckNormalRegime(t, r)
		})
	}
}

// Test 8 (g): a 503 on every leg (and on SSE) stays on the normal ladder and
// never enters auth-failed.
func TestSupervisor_NonAuthStatusStaysOnLadder(t *testing.T) {
	r := &t8Rig{t: t, sseStatus: 503}
	r.leg = func(leg, cycle int) t8Resp {
		if cycle == 1 {
			return t8Resp{status: 200, gen: 1}
		}
		return t8Resp{status: 503}
	}
	r.start(2)
	t8CheckNormalRegime(t, r)
}

// Test 8, polling mode (R2/R3): with SSE off the first rejected poll enters;
// the probe wait is max(T, poll interval). A single-leg client's only leg is
// the primary.
func TestSupervisor_AuthFailedPollingModeWaitsMaxOfPeriodAndInterval(t *testing.T) {
	for _, status := range t8AuthStatuses {
		for _, interval := range []time.Duration{60 * time.Second, 400 * time.Second} {
			t.Run(fmt.Sprintf("%d/interval-%s", status, interval), func(t *testing.T) {
				r := &t8Rig{t: t, sseStatus: 503}
				r.leg = func(leg, cycle int) t8Resp {
					switch {
					case cycle == 1:
						return t8Resp{status: 200, gen: 1}
					case cycle >= 5:
						return t8Resp{status: 200, gen: 2, exit: true}
					}
					return t8Resp{status: status}
				}
				c := r.start(1, WithSSE(false), WithFallbackPoll(true, interval))
				ev := r.await("exit", func(ev []t8Event) bool {
					return r.leg0Calls.Load() >= 6
				})
				if n := t8Count(ev, "hit:sse"); n != 0 {
					t.Errorf("%d SSE attempts in polling mode", n)
				}
				errs := t8Logs(ev, slog.LevelError)
				if len(errs) != 1 || !strings.Contains(errs[0], fmt.Sprint(status)) {
					t.Errorf("ERROR lines = %q, want exactly one naming %d", errs, status)
				}
				probes := t8Waits(ev, "probe")
				if len(probes) != 3 {
					t.Fatalf("probe waits = %v, want 3 (cycles 3, 4, 5)", probes)
				}
				for i, w := range probes {
					lo, hi := 150*time.Second, 300*time.Second
					if interval > hi {
						lo, hi = interval, interval
					}
					if w < lo || w > hi {
						t.Errorf("probe wait #%d = %s, want within [%s, %s]", i+1, w, lo, hi)
					}
				}
				enter := t8Index(ev, 0, "hit:0", 2)
				exitIdx := t8Index(ev, 0, "hit:0", 5)
				for _, e := range ev[enter+1 : exitIdx+1] {
					if e.kind == "hit:0" && (e.state != ConnStateDisconnected || e.active) {
						t.Errorf("during a probe: ConnectionState=%q FallbackPollerActive=%v, want disconnected/false", e.state, e.active)
					}
				}
				if got := c.HeldGeneration(); got != 2 {
					t.Errorf("HeldGeneration = %d, want 2", got)
				}
				if infos := t8Logs(ev[enter:], slog.LevelInfo); len(infos) != 1 {
					t.Errorf("INFO lines from entry = %q, want exactly one (exit)", infos)
				}
			})
		}
	}
}
