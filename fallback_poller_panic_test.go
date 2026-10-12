package quonfig

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// A panic in Fetch while the poller is engaged is recovered by the supervisor,
// which restarts Run. The restarted Run must not inherit the engaged flag:
// it starts with no poll ticker, so a stale active=true made engage() a no-op
// and the poller never polled again while SSE stayed down (qfg-goi1.10.14).
func TestFallbackPollerReengagesAfterRecoveredPanic(t *testing.T) {
	var fetches atomic.Int32
	var engages atomic.Int32
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  10 * time.Millisecond,
		Threshold: 10 * time.Millisecond,
		Fetch: func(ctx context.Context) error {
			if fetches.Add(1) == 1 {
				panic("fetch exploded while engaged")
			}
			return nil
		},
		OnEngage: func() { engages.Add(1) },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Minimal supervisor: recover a panic and restart Run.
		for ctx.Err() == nil {
			func() {
				defer func() { _ = recover() }()
				_ = p.Run(ctx)
			}()
		}
	}()

	// SSE never connects: threshold elapses, poller engages, first fetch panics.
	waitFor(t, time.Second, func() bool { return fetches.Load() >= 3 },
		"poller should re-engage and keep polling after a recovered panic")
	if got := engages.Load(); got < 2 {
		t.Errorf("OnEngage fired %d time(s), want >= 2 (re-engage after restart)", got)
	}
	if !p.Active() {
		t.Errorf("expected Active=true while re-engaged")
	}

	cancel()
	<-done
}
