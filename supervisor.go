package quonfig

// Supervisor pattern for sdk-go background workers.
//
// One supervisor instance per Client. It owns the Layer 1 (SSE) worker today
// and the Layer 2 (fallback poll) worker in a follow-up bead. Every worker
// runs inside defer recover() at the supervisor boundary: an unhandled panic
// or non-context error logs at ERROR, increments
// quonfig_sdk_worker_restart_total{layer="<n>"}, sleeps for an exponential
// backoff (500ms → 30s cap), and restarts. A clean exit driven by ctx
// cancellation is *not* counted as a restart.
//
// The supervisor is the source of truth for ConnectionState(). Workers
// report into it; callers read out of it. (LastSuccessfulRefresh lives on
// the Client — the init fetch runs before the supervisor exists, and
// successful-but-not-installed fetches must stamp it too; qfg-41nh.11.)
// See project/plans/sdk-hardening-and-verification.md §"Watcher of the
// watchers" for the full design.

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionState is the customer-visible health surface; values match the
// cross-SDK spec in project/plans/sdk-hardening-and-verification.md.
type ConnectionState string

const (
	// ConnStateInitializing is the pre-Start state and the state during the
	// first connection attempt before any worker has reported success.
	ConnStateInitializing ConnectionState = "initializing"
	// ConnStateConnected means an SSE stream (Layer 1) is live.
	ConnStateConnected ConnectionState = "connected"
	// ConnStateDisconnected means the Layer 1 worker is between connection
	// attempts (after a drop, before the next reconnect succeeds).
	ConnStateDisconnected ConnectionState = "disconnected"
	// ConnStateFallingBack means Layer 1 is unable to maintain a connection
	// and the Layer 2 fallback poller is active.
	ConnStateFallingBack ConnectionState = "falling_back"
)

// Worker is one supervised unit of background work. Run is invoked on a
// dedicated goroutine; the supervisor restarts it after any panic or non-
// context error. Run is expected to return promptly when ctx is cancelled.
type worker struct {
	// Layer is the metric label ("1" for SSE, "2" for fallback poll).
	Layer string
	// Run is the worker body. It must honor ctx and return when ctx is
	// cancelled. Any other exit is treated as a crash and triggers a restart.
	Run func(ctx context.Context) error
}

// supervisorConfig carries the knobs newSupervisor needs. Zero values get
// production defaults (500ms initial, 30s max, 5s stop deadline, default
// slog).
type supervisorConfig struct {
	Logger       *slog.Logger
	InitialDelay time.Duration
	MaxDelay     time.Duration
	StopTimeout  time.Duration
}

// Supervisor owns a set of long-running workers, restarts them on crash with
// exponential backoff, and exposes the health surface (ConnectionState,
// WorkerRestartTotal).
type supervisor struct {
	cfg     supervisorConfig
	workers []worker

	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startOnce sync.Once
	stopOnce  sync.Once

	mu           sync.Mutex
	state        ConnectionState
	restartTotal map[string]*atomic.Int64
}

func newSupervisor(cfg supervisorConfig, workers ...worker) *supervisor {
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 500 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.StopTimeout <= 0 {
		cfg.StopTimeout = 5 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &supervisor{
		cfg:          cfg,
		workers:      workers,
		ctx:          ctx,
		cancel:       cancel,
		state:        ConnStateInitializing,
		restartTotal: make(map[string]*atomic.Int64),
	}
}

// Start spawns every worker on its own goroutine. Idempotent.
func (s *supervisor) Start() {
	s.startOnce.Do(func() {
		for i := range s.workers {
			w := s.workers[i]
			s.wg.Add(1)
			go s.runWorker(w)
		}
	})
}

// Stop cancels the supervisor context (signaling every worker to wind down)
// and waits for all workers to exit, up to a 5s deadline. Idempotent.
func (s *supervisor) Stop() {
	s.stopOnce.Do(func() {
		s.cancel()
		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(s.cfg.StopTimeout):
			// A wedged worker (shouldn't happen — ctx is cancelled) must not
			// deadlock Close on the parent client.
			s.cfg.Logger.Warn("quonfig: supervisor.Stop deadline exceeded",
				slog.String("deadline", s.cfg.StopTimeout.String()),
			)
		}
	})
}

// runWorker is the per-worker outer loop: catch crashes, count them, back
// off, restart. Exits only when the supervisor context is cancelled.
func (s *supervisor) runWorker(w worker) {
	defer s.wg.Done()
	attempt := 0
	for {
		if s.ctx.Err() != nil {
			return
		}
		crashed := s.runOnce(w)
		if s.ctx.Err() != nil {
			return
		}
		if crashed {
			s.incRestartTotal(w.Layer)
		}
		// Backoff. Even a non-crashed (nil-return) early exit gets one tick
		// of delay so a runaway worker can't hot-loop.
		delay := s.backoffFor(attempt)
		attempt++

		t := time.NewTimer(delay)
		select {
		case <-s.ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// runOnce invokes w.Run inside a recover guard. Returns true if the worker
// crashed (panic or non-context error), false on clean ctx-cancelled exit.
func (s *supervisor) runOnce(w worker) (crashed bool) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		s.cfg.Logger.Error("quonfig: worker panicked; restarting",
			slog.String("layer", w.Layer),
			slog.Any("panic", r),
			slog.String("stack", string(debug.Stack())),
		)
		crashed = true
	}()
	err := w.Run(s.ctx)
	if err == nil {
		return false
	}
	if s.ctx.Err() != nil {
		// Cancellation in flight — caller's error is just the ctx error
		// surfacing; not a crash.
		return false
	}
	s.cfg.Logger.Error("quonfig: worker exited with error; restarting",
		slog.String("layer", w.Layer),
		slog.String("err", err.Error()),
	)
	return true
}

// backoffFor returns the sleep duration before the (attempt+1)th restart.
// 500ms → 1s → 2s → 4s → 8s → 16s → 30s (cap). Attempt is 0-indexed; the
// returned value is capped at MaxDelay.
func (s *supervisor) backoffFor(attempt int) time.Duration {
	d := s.cfg.InitialDelay
	for i := 0; i < attempt; i++ {
		next := d * 2
		if next >= s.cfg.MaxDelay || next < d /* overflow guard */ {
			return s.cfg.MaxDelay
		}
		d = next
	}
	if d > s.cfg.MaxDelay {
		return s.cfg.MaxDelay
	}
	return d
}

// WorkerRestartTotal returns the running count of
// quonfig_sdk_worker_restart_total{layer="<layer>"} for this supervisor.
// Unknown layers return 0.
func (s *supervisor) WorkerRestartTotal(layer string) int64 {
	s.mu.Lock()
	v := s.restartTotal[layer]
	s.mu.Unlock()
	if v == nil {
		return 0
	}
	return v.Load()
}

func (s *supervisor) incRestartTotal(layer string) {
	s.mu.Lock()
	v, ok := s.restartTotal[layer]
	if !ok {
		v = &atomic.Int64{}
		s.restartTotal[layer] = v
	}
	s.mu.Unlock()
	v.Add(1)
}

// ConnectionState returns the most recent transport state reported by any
// worker. Defaults to "initializing" before any state has been set.
func (s *supervisor) ConnectionState() ConnectionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// setConnectionState records a transport-state transition. Workers call this
// (e.g. the SSE worker on connect/disconnect, the fallback poller when it
// engages).
func (s *supervisor) setConnectionState(state ConnectionState) {
	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
}
