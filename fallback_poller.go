package quonfig

// Layer 2 fallback poller (bead qfg-47c2.20).
//
// The cross-SDK plan standardizes on fallback-only polling: while SSE is
// connected the poller is idle; when SSE has been disconnected for >=120s the
// poller engages and fetches at the configured interval; when SSE recovers
// the poller disengages.
//
// "Disconnected" includes "never connected": the engage timer is armed when
// the poller starts (unless SSE is already known to be connected), so a stream
// that never connects — primary outage at app start, a proxy that breaks
// streaming — still engages after Threshold (qfg-9dxb.2). When there is no SSE
// stream at all (WithSSE(false)) the poller engages immediately.
//
// fallbackPoller is wired into the supervisor as a Layer 2 worker. The
// supervisor owns its lifecycle (restart on panic, stop on Close); this type
// does not spawn its own goroutines.

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"sync/atomic"
	"time"
)

// DefaultFallbackPollThreshold is the disconnect window before Layer 2 engages.
// The cross-SDK spec calls for 120s; tests inject smaller values.
const DefaultFallbackPollThreshold = 120 * time.Second

// DefaultFallbackPollInterval is the cadence used once Layer 2 has engaged.
// Matches sdk-node/python/ruby/java; callers can override with WithFallbackPoll.
const DefaultFallbackPollInterval = 60 * time.Second

// The auth-failed probe period T is uniform in [authProbeMin, authProbeMax]
// (rule R3). Internal: tests reach it through the wait hook, never an option.
const (
	authProbeMin = 150 * time.Second
	authProbeMax = 300 * time.Second
)

// fallbackPollerConfig carries the knobs the Layer 2 worker needs.
type fallbackPollerConfig struct {
	// Interval is how often Fetch is called while engaged. Required (>0).
	Interval time.Duration
	// Threshold is the disconnect duration that triggers engagement. Defaults
	// to 120s when zero.
	Threshold time.Duration
	// NoStream means there is no SSE stream to wait for (SSE disabled): the
	// poller engages as soon as Run starts instead of after Threshold.
	NoStream bool
	// Fetch performs one poll. An *authRejectedError enters the auth-failed
	// regime (see Run); other errors are dropped and the poller keeps ticking
	// until ctx is cancelled or SSE reconnects.
	Fetch func(ctx context.Context) error
	// OnEngage / OnDisengage fire on the corresponding state-transition edges.
	// They run on the poller goroutine and should be cheap.
	OnEngage    func()
	OnDisengage func()
	// OnAuthFailed fires with true when a poll cycle enters auth-failed
	// (before the ERROR is logged) and with false when a probe exits it
	// (after the INFO). The client stops SSE on entry and restarts it on
	// exit. Runs on the poller goroutine.
	OnAuthFailed func(entered bool)
	// Logger receives debug-level state-transition logs. Falls back to
	// slog.Default() when nil.
	Logger *slog.Logger
	// Wait, if non-nil, maps each requested wait to the duration actually
	// waited (test seam, see Options.testWaitHook).
	Wait func(kind string, d time.Duration) time.Duration
}

type fallbackPoller struct {
	cfg     fallbackPollerConfig
	stateCh chan bool
	active  atomic.Bool
	// sseConnected is the most recent state passed to SetSSEConnected. Run
	// consults it at start so a supervisor restart while SSE is connected
	// does not arm the engage timer (no fresh edge would arrive to cancel it).
	sseConnected atomic.Bool
	// authFailed is set while the client is in the auth-failed regime. It
	// lives on the poller, not in Run, so a Run restarted by the supervisor
	// resumes probing instead of polling with SSE stopped.
	authFailed atomic.Bool
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
	p.sseConnected.Store(connected)
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

// wait applies the test wait hook, if any.
func (p *fallbackPoller) wait(kind string, d time.Duration) time.Duration {
	if p.cfg.Wait == nil {
		return d
	}
	return p.cfg.Wait(kind, d)
}

// Active reports whether the poller is currently in the engaged state (i.e.
// actively polling because SSE has been down past Threshold).
func (p *fallbackPoller) Active() bool {
	return p.active.Load()
}

// authProbeWait returns the wait before the next auth-failed probe: T uniform
// in [AuthProbeMin, AuthProbeMax], fixed (no doubling). With no SSE stream
// (polling mode) the wait is max(T, poll interval), rule R3.
func (p *fallbackPoller) authProbeWait() time.Duration {
	d := authProbeMin + time.Duration(rand.Int63n(int64(authProbeMax-authProbeMin)+1))
	if p.cfg.NoStream && p.cfg.Interval > d {
		d = p.cfg.Interval
	}
	return d
}

// AuthFailed reports whether the client is in the auth-failed regime.
func (p *fallbackPoller) AuthFailed() bool {
	return p.authFailed.Load()
}

// Run is the worker body. Suitable for supervisor.worker.Run — it honors ctx
// and returns nil on cancellation. It never returns a non-nil error today.
//
// Auth-failure rule (qfg-goi1.10.23, plan 2026-10-07-sdk-auth-failure-rule.md
// R2-R5): when one of the poller's fetch cycles comes back with no 200/304
// on any leg and an auth-class status (401/403/404/410) on the primary, the
// poller enters auth-failed. It stops polling, has the client stop SSE
// (OnAuthFailed(true)), logs one ERROR, and from then on sends one probe
// cycle per T uniform in [150s, 300s] for as long as the process lives (no
// doubling, no latch). The first probe that gets a 200 or 304 on any leg
// exits: one INFO, OnAuthFailed(false) restarts SSE, and the poller returns
// to its normal engage rule.
func (p *fallbackPoller) Run(ctx context.Context) error {
	// Run starts with no poll ticker, so it must start disengaged. A previous
	// Run that panicked while engaged (recovered and restarted by the
	// supervisor) left active=true, which made engage() a no-op and wedged the
	// poller for the rest of the outage (qfg-goi1.10.14).
	if p.active.Swap(false) {
		p.cfg.Logger.Warn("quonfig: Layer 2 fallback poller restarted while engaged; re-arming")
	}

	var engageTimer *time.Timer
	var pollTicker *time.Ticker
	var probeTimer *time.Timer
	defer func() {
		if engageTimer != nil {
			engageTimer.Stop()
		}
		if pollTicker != nil {
			pollTicker.Stop()
		}
		if probeTimer != nil {
			probeTimer.Stop()
		}
	}()

	armProbe := func() {
		probeTimer = time.NewTimer(p.wait("probe", p.authProbeWait()))
	}

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

	// fetch runs one ordinary poll cycle and enters auth-failed when the
	// cycle qualifies (R2). Reports whether it entered.
	fetch := func() bool {
		if p.cfg.Fetch == nil {
			return false
		}
		err := p.cfg.Fetch(ctx)
		var rejected *authRejectedError
		if ctx.Err() != nil || !errors.As(err, &rejected) {
			return false
		}
		if pollTicker != nil {
			pollTicker.Stop()
			pollTicker = nil
		}
		if engageTimer != nil {
			engageTimer.Stop()
			engageTimer = nil
		}
		p.active.Store(false)
		p.authFailed.Store(true)
		if p.cfg.OnAuthFailed != nil {
			p.cfg.OnAuthFailed(true)
		}
		p.cfg.Logger.Error("quonfig: config fetch rejected: the SDK key is invalid or revoked (check the SDK key); streaming and polling stopped, retrying every few minutes until it is accepted",
			slog.Int("status", rejected.Status),
			slog.String("host", rejected.Host),
		)
		armProbe()
		return true
	}

	engage := func() {
		if p.active.CompareAndSwap(false, true) {
			if p.cfg.OnEngage != nil {
				p.cfg.OnEngage()
			}
			// Immediate fetch on engagement so the first refresh after the
			// 120s window doesn't wait another full interval.
			if fetch() {
				return
			}
			if p.cfg.Interval > 0 {
				pollTicker = time.NewTicker(p.wait("poll", p.cfg.Interval))
			}
		}
	}

	// probe runs one auth-failed probe cycle; the first 200/304 on any leg
	// (Fetch returns nil) exits auth-failed (R4).
	probe := func() {
		if p.cfg.Fetch == nil {
			armProbe()
			return
		}
		err := p.cfg.Fetch(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			p.cfg.Logger.Debug("quonfig: auth-failed probe rejected; retrying later",
				slog.String("err", err.Error()),
			)
			armProbe()
			return
		}
		p.authFailed.Store(false)
		p.cfg.Logger.Info("quonfig: config fetch accepted; leaving auth-failed and resuming streaming and polling")
		if p.cfg.OnAuthFailed != nil {
			p.cfg.OnAuthFailed(false)
		}
		if p.cfg.NoStream {
			// Polling mode: resume polling at the configured interval. The
			// probe just fetched, so no immediate fetch.
			p.active.Store(true)
			if p.cfg.Interval > 0 {
				pollTicker = time.NewTicker(p.wait("poll", p.cfg.Interval))
			}
			return
		}
		// SSE restarts from its base backoff (OnAuthFailed(false)); the
		// poller is back on its normal engage rule.
		if !p.sseConnected.Load() {
			engageTimer = time.NewTimer(p.wait("engage", p.cfg.Threshold))
		}
	}

	// Arm engagement at start. A never-connecting stream produces no
	// connected->disconnected edge, so waiting for one would leave the poller
	// idle forever and config frozen at the init snapshot. A Run restarted by
	// the supervisor while auth-failed stays auth-failed.
	switch {
	case p.authFailed.Load():
		armProbe()
	case p.cfg.NoStream:
		engage()
	case !p.sseConnected.Load():
		engageTimer = time.NewTimer(p.wait("engage", p.cfg.Threshold))
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
		var probeC <-chan time.Time
		if probeTimer != nil {
			probeC = probeTimer.C
		}

		select {
		case <-ctx.Done():
			return nil
		case connected := <-p.stateCh:
			if p.authFailed.Load() {
				// SSE is stopped while auth-failed; a late edge from the
				// stream being torn down changes nothing.
				continue
			}
			if connected {
				if engageTimer != nil {
					engageTimer.Stop()
					engageTimer = nil
				}
				disengage()
			} else {
				if !p.active.Load() && engageTimer == nil {
					engageTimer = time.NewTimer(p.wait("engage", p.cfg.Threshold))
				}
			}
		case <-engageC:
			engageTimer = nil
			engage()
		case <-pollC:
			fetch()
		case <-probeC:
			probeTimer = nil
			probe()
		}
	}
}
