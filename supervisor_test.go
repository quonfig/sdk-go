package quonfig

import (
	"context"
	"errors"
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
//   - exposes ConnectionState() and LastSuccessfulRefresh() for callers.
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

// Test 5 — A panic that surfaces *from* the worker (e.g. a panic that the
// in-worker OnEnvelope recover() failed to catch, or a future worker shape
// that re-raises) is caught by the supervisor as a last line of defense.
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
// worker reports state changes; LastSuccessfulRefresh advances when the
// worker records an install. The supervisor itself does not own the
// transport — it provides the surface that workers report into.
func TestSupervisorConnectionStateAndLastRefresh(t *testing.T) {
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

	// Before Start: initializing, zero refresh.
	if got, want := s.ConnectionState(), ConnStateInitializing; got != want {
		t.Errorf("pre-Start ConnectionState = %q, want %q", got, want)
	}
	if !s.LastSuccessfulRefresh().IsZero() {
		t.Errorf("pre-Start LastSuccessfulRefresh should be zero, got %s", s.LastSuccessfulRefresh())
	}

	s.Start()
	defer s.Stop()

	// Drive state transitions the way a worker would.
	s.setConnectionState(ConnStateConnected)
	if got, want := s.ConnectionState(), ConnStateConnected; got != want {
		t.Errorf("after connected ConnectionState = %q, want %q", got, want)
	}

	before := time.Now()
	s.recordSuccessfulRefresh()
	got := s.LastSuccessfulRefresh()
	if got.Before(before) || got.After(time.Now().Add(1*time.Second)) {
		t.Errorf("LastSuccessfulRefresh = %s, want between %s and now", got, before)
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
