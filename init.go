package quonfig

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Startup-failure behavior (qfg-9p4i.7). Spec:
// project/plans/2026-10-05-init-failure-design.md, sections A.3, B and C.
//
// NewClient blocks while it fetches the first config. Each attempt is one
// hedged primary+secondary fetch; between attempts it backs off 0.5s, 1s, 2s,
// ... capped at 30s, with jitter. When InitTimeout runs out without a config
// (or every leg rejects the SDK key) the OnInitFailure policy decides:
//
//   - fail:     NewClient returns (nil, *InitError); no client exists, so the
//     orchestrator's restart is the retry.
//   - wait:     NewClient keeps blocking and retrying until a config arrives.
//   - continue: NewClient returns a client with an empty store. The retry loop
//     keeps running in the background (as do SSE and the fallback poller) until
//     the client heals.

// InitErrorReason says why NewClient gave up under InitFailureFail.
type InitErrorReason string

const (
	// InitErrorTimeout: the last attempt was still waiting on delivery when
	// InitTimeout ran out (the legs hang). errors.Is(err,
	// ErrInitializationTimeout) holds only for this reason.
	InitErrorTimeout InitErrorReason = "timeout"
	// InitErrorUnavailable: every attempt failed (connection refused, DNS,
	// 5xx, ...) until InitTimeout ran out.
	InitErrorUnavailable InitErrorReason = "unavailable"
	// InitErrorUnauthorized: every delivery leg rejected the SDK key
	// (401/403/404/410). Returned at once, without waiting out InitTimeout.
	InitErrorUnauthorized InitErrorReason = "unauthorized"
)

// InitError is the error NewClient returns under InitFailureFail. Unwrap gives
// the last delivery error; errors.Is(err, ErrInitializationTimeout) is true
// when Reason is InitErrorTimeout.
type InitError struct {
	Reason InitErrorReason
	// Elapsed is how long NewClient tried before giving up.
	Elapsed time.Duration
	// Attempts is the number of hedged fetch attempts made.
	Attempts int
	// Err is the last delivery error (may be nil).
	Err error

	legs string // "primary=<host>: <err>; secondary=<host>: <err>"
}

func (e *InitError) Error() string {
	var b strings.Builder
	switch e.Reason {
	case InitErrorUnauthorized:
		b.WriteString("quonfig: init failed: SDK key rejected by every delivery leg")
	case InitErrorTimeout:
		fmt.Fprintf(&b, "%s: quonfig: init failed: no config after %.1fs", ErrInitializationTimeout.Error(), e.Elapsed.Seconds())
	default:
		fmt.Fprintf(&b, "quonfig: init failed: no config after %.1fs (delivery unavailable)", e.Elapsed.Seconds())
	}
	if e.legs != "" {
		b.WriteString(". ")
		b.WriteString(e.legs)
	}
	return b.String()
}

// Unwrap returns the last delivery error.
func (e *InitError) Unwrap() error { return e.Err }

// Is reports ErrInitializationTimeout for a timeout, so the pre-existing
// errors.Is check keeps working on a hang.
func (e *InitError) Is(target error) bool {
	return target == ErrInitializationTimeout && e.Reason == InitErrorTimeout
}

// errNotReady is what getters return on a client that has no config yet
// (InitFailureContinue before it heals). It matches ErrNotFound, so it reads as
// an empty store, and maps to ErrorCodeProviderNotReady.
type notReadyError struct{ key string }

func (e *notReadyError) Error() string {
	return fmt.Sprintf("%s: %q: client has no config yet (onInitFailure=continue, still retrying)", ErrNotFound.Error(), e.key)
}
func (e *notReadyError) Is(target error) bool { return target == ErrNotFound }

// errClientClosed is returned by WaitForReady after Close.
var errClientClosed = errors.New("quonfig: client closed")

// Ready reports whether the client holds a config: true once any config has
// been installed (or for a client with no delivery: datadir or no SDK key).
// Under InitFailureContinue it is false until the background retry heals.
// Wire readiness probes to it.
func (c *Client) Ready() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.transport == nil || c.initialized
}

// WaitForReady blocks until the client holds a config (see Ready), ctx is
// done (returns ctx.Err()), or the client is closed.
func (c *Client) WaitForReady(ctx context.Context) error {
	if c.Ready() {
		return nil
	}
	select {
	case <-c.initializationDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closeCh:
		if c.Ready() {
			return nil
		}
		return errClientClosed
	}
}

// Backoff between init attempts (B.3). Package vars so tests can shrink them.
var (
	initBackoffBase = 500 * time.Millisecond
	initBackoffMax  = 30 * time.Second
)

// Log cadences (C). Package vars so tests can shrink them.
var (
	initProgressEvery      = 5 * time.Second  // until the timeout
	initWaitProgressEvery  = 30 * time.Second // wait, after the timeout
	initBackgroundEvery    = 30 * time.Second // continue, first 5 minutes
	initBackgroundSlowFrom = 5 * time.Minute
	initBackgroundSlow     = 5 * time.Minute
)

// deprecatedInitAliasWarn logs the one-time WARN per deprecated alias.
var deprecatedInitAliasWarn sync.Map

func warnDeprecatedInitAlias(logger *slog.Logger, f OnInitFailure) {
	var oldName, newName string
	switch f {
	case ReturnError:
		oldName, newName = "ReturnError", "InitFailureFail"
	case ReturnZeroValue:
		oldName, newName = "ReturnZeroValue", "InitFailureContinue"
	default:
		return
	}
	if _, loaded := deprecatedInitAliasWarn.LoadOrStore(f, true); loaded {
		return
	}
	logger.Warn(fmt.Sprintf("[quonfig] WithOnInitFailure(quonfig.%s) is deprecated; use quonfig.%s", oldName, newName),
		slog.String("event", "quonfig.init.deprecated_alias"))
}

// initAttempt is the outcome of one hedged fetch cycle on the init path.
type initAttempt struct {
	installed bool
	err       error  // last leg error
	legs      string // <errs> per the logging contract
	allAuth   bool   // every configured leg answered auth-class
	authCode  int    // the primary's auth-class status when allAuth
	hung      bool   // an attempt leg ended on the init deadline
}

// runInitAttempt runs one hedged fetch cycle and summarizes it.
func (c *Client) runInitAttempt(ctx context.Context) initAttempt {
	out, err := c.fetchCycle(ctx)
	a := initAttempt{installed: out.installed, err: err}
	if out.installed {
		return a
	}
	var parts []string
	auth := 0
	for _, l := range out.legs {
		name := "primary"
		if l.index > 0 {
			name = "secondary"
		}
		host := ""
		if l.index >= 0 && l.index < len(c.transport.baseURLs) {
			host = hostOf(c.transport.baseURLs[l.index])
		}
		msg := "ok"
		if l.err != nil {
			msg = legErrorText(l)
		}
		parts = append(parts, fmt.Sprintf("%s=%s: %s", name, host, msg))
		if l.err != nil && isAuthClassStatus(l.status) {
			auth++
			if l.index == 0 || a.authCode == 0 {
				a.authCode = l.status
			}
		}
	}
	a.legs = strings.Join(parts, "; ")
	// A hang: every leg that answered at all ended on a deadline (the init
	// deadline or the per-leg hedge abort), none with a fast error.
	a.hung = len(out.legs) > 0
	for _, l := range out.legs {
		if l.err == nil || !isTimeoutErr(l.err) {
			a.hung = false
		}
	}
	a.allAuth = len(out.legs) > 0 && auth == len(c.transport.baseURLs)
	return a
}

func isTimeoutErr(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne interface{ Timeout() bool }
	return errors.As(err, &ne) && ne.Timeout()
}

// legErrorText is the short per-leg error for the log lines: the status for
// an HTTP answer, else the transport error. Never carries the SDK key (the
// key travels in the Authorization header, which errors do not echo).
func legErrorText(l legOutcome) string {
	if l.status != 0 {
		return fmt.Sprintf("HTTP %d", l.status)
	}
	if isTimeoutErr(l.err) {
		return "timed out"
	}
	if errors.Is(l.err, context.Canceled) {
		return "canceled"
	}
	msg := l.err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 && i+2 < len(msg) {
		// "fetching configs from <url>: Get ...: dial tcp ...: connection refused" -> the cause.
		msg = msg[i+2:]
	}
	return msg
}

// initState is shared between the init loop and its reporter goroutine.
type initState struct {
	cad initCadence // read once at construction; immutable

	mu            sync.Mutex
	start         time.Time
	attempts      int
	legs          string
	failureLogged bool // a WARN+ init-failure line was emitted
	phase         initPhase
	phaseAt       time.Time
}

// initCadence is a snapshot of the backoff and log cadence package vars.
type initCadence struct {
	backoffBase, backoffMax            time.Duration
	progress, waitProgress, background time.Duration
	backgroundSlowFrom, backgroundSlow time.Duration
}

func currentInitCadence() initCadence {
	return initCadence{
		backoffBase: initBackoffBase, backoffMax: initBackoffMax,
		progress: initProgressEvery, waitProgress: initWaitProgressEvery, background: initBackgroundEvery,
		backgroundSlowFrom: initBackgroundSlowFrom, backgroundSlow: initBackgroundSlow,
	}
}

type initPhase int

const (
	phaseWaiting    initPhase = iota // before the timeout
	phaseWaitPast                    // wait, after the timeout
	phaseBackground                  // continue, returned without config
	phaseDone
)

func (s *initState) snapshot() (attempts int, legs string, elapsed time.Duration, phase initPhase, phaseAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempts, s.legs, time.Since(s.start), s.phase, s.phaseAt
}

func (s *initState) setPhase(p initPhase) {
	s.mu.Lock()
	s.phase = p
	s.phaseAt = time.Now()
	s.mu.Unlock()
}

func fmtSeconds(d time.Duration) string { return fmt.Sprintf("%.1fs", d.Seconds()) }

func fmtTimeout(d time.Duration) string {
	if d%time.Second == 0 {
		return fmt.Sprintf("%ds", int(d/time.Second))
	}
	return d.String()
}

// initialize runs the init loop and blocks until the OnInitFailure policy has
// decided. A non-nil error is the *InitError for InitFailureFail; the caller
// must then Close the client.
func (c *Client) initialize() error {
	mode, _ := c.opts.OnInitFailure.canonical()
	timeout := c.opts.InitTimeout
	logger := c.opts.Logger

	logger.Debug(fmt.Sprintf("[quonfig] init: fetching config (initTimeout=%s, onInitFailure=%s)", fmtTimeout(timeout), mode),
		slog.String("event", "quonfig.init.start"))

	st := &initState{start: time.Now(), cad: currentInitCadence()}
	st.phaseAt = st.start

	// base is cancelled on Close, so a background retry (or a wait) stops.
	base, cancelBase := context.WithCancel(context.Background())
	go func() {
		select {
		case <-c.closeCh:
		case <-base.Done():
		}
		cancelBase()
	}()

	decision := make(chan error, 1)
	go func() {
		// The loop ends once the client holds a config, is closed, or failed;
		// release the Close watcher with it.
		defer cancelBase()
		c.initLoop(base, mode, timeout, st, decision)
	}()
	// Return as soon as the first config is installed, before that install's
	// OnConfigUpdate runs (it is delivered on the init goroutine), so a
	// callback that uses the client returned by NewClient cannot deadlock
	// against it. The init goroutine goes on to drain the hedge (a late, newer
	// leg still heals forward) and start the background workers.
	select {
	case err := <-decision:
		return err
	case <-c.initializationDone:
		return nil
	}
}

func (c *Client) initLoop(base context.Context, mode OnInitFailure, timeout time.Duration, st *initState, decision chan<- error) {
	logger := c.opts.Logger
	decided := false
	timedOut := false
	reporter := false
	decide := func(err error) {
		if !decided {
			decided = true
			decision <- err
		}
	}
	defer func() {
		st.setPhase(phaseDone)
		if !decided {
			// Closed before deciding (cannot happen through NewClient, which
			// holds the only reference until it returns).
			decide(errClientClosed)
		}
	}()

	// The attempt context carries the init deadline until the policy has
	// decided; wait and the background retry run with only Close as a bound.
	var deadlineCtx context.Context = base
	var deadlineCh <-chan struct{}
	if timeout > 0 {
		var cancel context.CancelFunc
		deadlineCtx, cancel = context.WithTimeout(base, timeout)
		defer cancel()
		deadlineCh = deadlineCtx.Done()
	}

	startReporter := func() {
		if !reporter {
			reporter = true
			go c.initReporter(st)
		}
	}

	backoff := st.cad.backoffBase
	var last initAttempt
	attemptsSoFar := 0
	for {
		if c.Ready() {
			// SSE or the fallback poller healed a continue client.
			return
		}
		// The deadline may have passed while we slept (the backoff timer and
		// the deadline can fire together); decide before another attempt.
		if !timedOut && timeout > 0 && deadlineCtx.Err() != nil {
			timedOut = true
			if c.onInitTimeout(mode, timeout, st, last, attemptsSoFar, decide) {
				return
			}
			startReporter()
		}
		actx := base
		if !timedOut && mode != InitFailureWait {
			actx = deadlineCtx
		}
		a := c.runInitAttempt(actx)
		st.mu.Lock()
		st.attempts++
		attempts := st.attempts
		attemptsSoFar = attempts
		if !a.installed && a.legs != "" {
			st.legs = a.legs
		}
		st.mu.Unlock()

		if a.installed {
			if !decided {
				c.startBackgroundWorkers()
			}
			decide(nil)
			return
		}
		if base.Err() != nil {
			return
		}
		last = a

		// Every leg rejected the key: fail and continue decide now; wait
		// keeps retrying (the key may be provisioned late).
		if a.allAuth && !decided {
			switch mode {
			case InitFailureFail:
				logger.Error(fmt.Sprintf("[quonfig] init FAILED: SDK key rejected (%d) by primary and secondary. Check the key and environment. onInitFailure=fail: returning *quonfig.InitError now.", a.authCode),
					slog.String("event", "quonfig.init.unauthorized"))
				decide(&InitError{Reason: InitErrorUnauthorized, Elapsed: time.Since(st.start), Attempts: attempts, Err: a.err, legs: a.legs})
				return
			case InitFailureContinue:
				logger.Error(fmt.Sprintf("[quonfig] init FAILED: SDK key rejected (%d) by primary and secondary. Check the key and environment. onInitFailure=continue: STARTING ON CODE DEFAULTS. This instance may disagree with the rest of your fleet. Retrying in background.", a.authCode),
					slog.String("event", "quonfig.init.unauthorized"))
				st.mu.Lock()
				st.failureLogged = true
				st.mu.Unlock()
				timedOut = true
				st.setPhase(phaseBackground)
				startReporter()
				c.startBackgroundWorkers()
				decide(nil)
			case InitFailureWait:
				if attempts == 1 {
					logger.Error(fmt.Sprintf("[quonfig] init: SDK key rejected (%d) by primary and secondary. Check the key and environment. onInitFailure=wait: still blocking startup and retrying.", a.authCode),
						slog.String("event", "quonfig.init.unauthorized"))
					st.mu.Lock()
					st.failureLogged = true
					st.mu.Unlock()
				}
			}
		}

		if attempts == 1 && !decided && !(timeout <= 0) {
			st.mu.Lock()
			already := st.failureLogged
			st.failureLogged = true
			st.mu.Unlock()
			if !already {
				logger.Warn(fmt.Sprintf("[quonfig] init: config not loaded yet, retrying until initTimeout (%s). %s", fmtTimeout(timeout), a.legs),
					slog.String("event", "quonfig.init.retrying"))
			}
			startReporter()
		}

		// The timeout may have fired during the attempt (a hang).
		if !timedOut && (timeout <= 0 || deadlineCtx.Err() != nil) {
			timedOut = true
			if c.onInitTimeout(mode, timeout, st, last, attempts, decide) {
				return
			}
			startReporter()
		}

		wait := jitter(backoff)
		backoff *= 2
		if backoff > st.cad.backoffMax {
			backoff = st.cad.backoffMax
		}
		timer := time.NewTimer(wait)
		var dl <-chan struct{}
		if !timedOut {
			dl = deadlineCh
		}
		select {
		case <-timer.C:
		case <-c.initializationDone:
			timer.Stop()
			return
		case <-base.Done():
			timer.Stop()
			return
		case <-dl:
			timer.Stop()
			timedOut = true
			if c.onInitTimeout(mode, timeout, st, last, attempts, decide) {
				return
			}
			startReporter()
		}
	}
}

// onInitTimeout applies the policy at the timeout. It returns true when the
// loop must stop (fail).
func (c *Client) onInitTimeout(mode OnInitFailure, timeout time.Duration, st *initState, last initAttempt, attempts int, decide func(error)) bool {
	logger := c.opts.Logger
	elapsed := time.Since(st.start)
	st.mu.Lock()
	st.failureLogged = true
	st.mu.Unlock()
	switch mode {
	case InitFailureWait:
		logger.Warn(fmt.Sprintf("[quonfig] init: no config after %s. onInitFailure=wait: still blocking startup and retrying indefinitely. %s. Use onInitFailure=fail or continue to bound startup.", fmtSeconds(elapsed), last.legs),
			slog.String("event", "quonfig.init.wait"))
		st.setPhase(phaseWaitPast)
		return false
	case InitFailureContinue:
		logger.Error(fmt.Sprintf("[quonfig] init: no config after %s. onInitFailure=continue: STARTING ON CODE DEFAULTS. This instance may disagree with the rest of your fleet. Retrying in background.", fmtSeconds(elapsed)),
			slog.String("event", "quonfig.init.continue"))
		st.setPhase(phaseBackground)
		c.startBackgroundWorkers()
		decide(nil)
		return false
	default:
		reason := InitErrorUnavailable
		if last.hung || attempts == 0 {
			reason = InitErrorTimeout
		}
		logger.Error(fmt.Sprintf("[quonfig] init FAILED: no config after %s. %s. onInitFailure=fail: returning *quonfig.InitError. Options: onInitFailure=wait to keep waiting, onInitFailure=continue to start on code defaults, or raise initTimeout.", fmtSeconds(elapsed), last.legs),
			slog.String("event", "quonfig.init.failed"))
		decide(&InitError{Reason: reason, Elapsed: elapsed, Attempts: attempts, Err: last.err, legs: last.legs})
		return true
	}
}

// initReporter logs the periodic progress lines and the heal line (C).
func (c *Client) initReporter(st *initState) {
	logger := c.opts.Logger
	lastLog := time.Now()
	tick := time.NewTicker(minDuration(st.cad.progress, st.cad.waitProgress, st.cad.background) / 5)
	defer tick.Stop()
	for {
		select {
		case <-c.closeCh:
			return
		case <-c.initializationDone:
			st.mu.Lock()
			logged := st.failureLogged
			start := st.start
			st.mu.Unlock()
			if logged {
				c.mu.RLock()
				via := "primary"
				if c.resolvedFromIndex > 0 {
					via = "secondary"
				}
				version := c.heldVersion
				c.mu.RUnlock()
				logger.Warn(fmt.Sprintf("[quonfig] init recovered: config loaded after %s via %s (version %s). Now serving live config.", fmtSeconds(time.Since(start)), via, version),
					slog.String("event", "quonfig.init.recovered"))
			}
			return
		case <-tick.C:
		}
		attempts, legs, elapsed, phase, phaseAt := st.snapshot()
		since := time.Since(lastLog)
		switch phase {
		case phaseWaiting:
			if since >= st.cad.progress && elapsed < c.opts.InitTimeout {
				logger.Warn(fmt.Sprintf("[quonfig] init: still waiting for config (%s elapsed of %s, attempt %d). %s", fmtSeconds(elapsed), fmtTimeout(c.opts.InitTimeout), attempts, legs),
					slog.String("event", "quonfig.init.waiting"))
				lastLog = time.Now()
			}
		case phaseWaitPast:
			if since >= st.cad.waitProgress {
				logger.Warn(fmt.Sprintf("[quonfig] init: still blocked waiting for config (%s elapsed, attempt %d). %s", fmtElapsed(elapsed), attempts, legs),
					slog.String("event", "quonfig.init.waiting"))
				lastLog = time.Now()
			}
		case phaseBackground:
			every := st.cad.background
			if time.Since(phaseAt) >= st.cad.backgroundSlowFrom {
				every = st.cad.backgroundSlow
			}
			if since >= every {
				logger.Warn(fmt.Sprintf("[quonfig] still not connected to Quonfig delivery (%s since start); serving defaults. %s", fmtElapsed(elapsed), legs),
					slog.String("event", "quonfig.init.not_connected"))
				lastLog = time.Now()
			}
		case phaseDone:
			return
		}
	}
}

func minDuration(ds ...time.Duration) time.Duration {
	m := ds[0]
	for _, d := range ds[1:] {
		if d < m {
			m = d
		}
	}
	if m <= 0 {
		m = time.Second
	}
	return m
}

// fmtElapsed renders "70s" / "5m30s" for the long-running lines.
func fmtElapsed(d time.Duration) string {
	return d.Round(time.Second).String()
}

func jitter(d time.Duration) time.Duration {
	// +/-20%.
	f := 0.8 + 0.4*rand.Float64() //nolint:gosec // jitter, not security
	return time.Duration(float64(d) * f)
}
