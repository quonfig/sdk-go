package quonfig

// Layer 2 fallback poller (bead qfg-47c2.20).
//
// The cross-SDK plan standardizes on fallback-only polling: while SSE is
// connected the poller is idle; when SSE has been disconnected for >=120s the
// poller engages and fetches at the configured interval; when SSE recovers
// the poller disengages.
//
// fallbackPoller is wired into the supervisor as a Layer 2 worker. The
// supervisor owns its lifecycle (restart on panic, stop on Close); this type
// does not spawn its own goroutines.

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// DefaultFallbackPollThreshold is the disconnect window before Layer 2 engages.
// The cross-SDK spec calls for 120s; tests inject smaller values.
const DefaultFallbackPollThreshold = 120 * time.Second

// DefaultFallbackPollInterval is the cadence used once Layer 2 has engaged.
// Matches sdk-node/python/ruby/java; callers can override with WithFallbackPoll.
const DefaultFallbackPollInterval = 60 * time.Second

// fallbackPollerConfig carries the knobs the Layer 2 worker needs.
type fallbackPollerConfig struct {
	// Interval is how often Fetch is called while engaged. Required (>0).
	Interval time.Duration
	// Threshold is the disconnect duration that triggers engagement. Defaults
	// to 120s when zero.
	Threshold time.Duration
	// Fetch performs one poll. Returned errors are logged at debug; the
	// poller keeps ticking until ctx is cancelled or SSE reconnects.
	Fetch func(ctx context.Context) error
	// OnEngage / OnDisengage fire on the corresponding state-transition edges.
	// They run on the poller goroutine and should be cheap.
	OnEngage    func()
	OnDisengage func()
	// Logger receives debug-level state-transition logs. Falls back to
	// slog.Default() when nil.
	Logger *slog.Logger
}

type fallbackPoller struct {
	cfg     fallbackPollerConfig
	stateCh chan bool
	active  atomic.Bool
}

func newFallbackPoller(cfg fallbackPollerConfig) *fallbackPoller {
	if cfg.Threshold <= 0 {
		cfg.Threshold = DefaultFallbackPollThreshold
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &fallbackPoller{
		cfg: cfg,
		// Buffer 1 so SetSSEConnected never blocks the caller (the SSE state
		// callback). A coalesce-newest semantic is fine: a stale "true" in the
		// queue followed by another "true" is harmless, and the poller drains
		// promptly.
		stateCh: make(chan bool, 1),
	}
}

// SetSSEConnected feeds an SSE connection state edge into the poller. Safe to
// call from any goroutine, never blocks.
func (p *fallbackPoller) SetSSEConnected(connected bool) {
	// Drain stale entries so the most recent state wins. With a buffer of 1
	// this loop runs at most once.
	for {
		select {
		case p.stateCh <- connected:
			return
		default:
			select {
			case <-p.stateCh:
			default:
				return
			}
		}
	}
}

// Active reports whether the poller is currently in the engaged state (i.e.
// actively polling because SSE has been down past Threshold).
func (p *fallbackPoller) Active() bool {
	return p.active.Load()
}

// Run is the worker body. Suitable for supervisor.worker.Run — it honors ctx
// and returns nil on cancellation. It never returns a non-nil error today.
func (p *fallbackPoller) Run(ctx context.Context) error {
	var engageTimer *time.Timer
	var pollTicker *time.Ticker
	defer func() {
		if engageTimer != nil {
			engageTimer.Stop()
		}
		if pollTicker != nil {
			pollTicker.Stop()
		}
	}()

	disengage := func() {
		if p.active.CompareAndSwap(true, false) {
			if pollTicker != nil {
				pollTicker.Stop()
				pollTicker = nil
			}
			if p.cfg.OnDisengage != nil {
				p.cfg.OnDisengage()
			}
		}
	}

	engage := func() {
		if p.active.CompareAndSwap(false, true) {
			if p.cfg.OnEngage != nil {
				p.cfg.OnEngage()
			}
			// Immediate fetch on engagement so the first refresh after the
			// 120s window doesn't wait another full interval.
			if p.cfg.Fetch != nil {
				_ = p.cfg.Fetch(ctx)
			}
			if p.cfg.Interval > 0 {
				pollTicker = time.NewTicker(p.cfg.Interval)
			}
		}
	}

	for {
		var engageC <-chan time.Time
		if engageTimer != nil {
			engageC = engageTimer.C
		}
		var pollC <-chan time.Time
		if pollTicker != nil {
			pollC = pollTicker.C
		}

		select {
		case <-ctx.Done():
			return nil
		case connected := <-p.stateCh:
			if connected {
				if engageTimer != nil {
					engageTimer.Stop()
					engageTimer = nil
				}
				disengage()
			} else {
				if !p.active.Load() && engageTimer == nil {
					engageTimer = time.NewTimer(p.cfg.Threshold)
				}
			}
		case <-engageC:
			engageTimer = nil
			engage()
		case <-pollC:
			if p.cfg.Fetch != nil {
				_ = p.cfg.Fetch(ctx)
			}
		}
	}
}
