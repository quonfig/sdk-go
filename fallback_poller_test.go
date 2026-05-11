package quonfig

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fallbackPoller is the Layer 2 worker. Behavior contract:
//
//  - It is idle while SSE is connected.
//  - On SSE disconnect, it starts a "threshold" timer (default 120s).
//  - If SSE reconnects before the threshold elapses, the timer is cancelled
//    and the poller stays idle. No fetch happens.
//  - If the threshold elapses while still disconnected, the poller engages:
//    it fires an immediate fetch and then ticks at the configured interval
//    calling fetch each tick.
//  - When SSE reconnects (or context is cancelled), the poller disengages
//    and returns to idle.
//
// The poller is wired into the supervisor as a Layer 2 worker. The supervisor
// owns the lifecycle (restart on panic, stop on Close); fallbackPoller does
// not spawn its own goroutines.

// helper: wait for predicate or fail after timeout.
func waitFor(t *testing.T, d time.Duration, predicate func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("waitFor timed out: %s", msg)
}

func TestFallbackPollerIdleWhileConnected(t *testing.T) {
	var fetches atomic.Int32
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  10 * time.Millisecond,
		Threshold: 10 * time.Millisecond,
		Fetch: func(ctx context.Context) error {
			fetches.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = p.Run(ctx)
		close(done)
	}()

	p.SetSSEConnected(true)
	time.Sleep(60 * time.Millisecond)

	if got := fetches.Load(); got != 0 {
		t.Errorf("expected 0 fetches while connected, got %d", got)
	}
	if p.Active() {
		t.Errorf("expected Active=false while connected")
	}

	cancel()
	<-done
}

func TestFallbackPollerEngagesAfterThreshold(t *testing.T) {
	var fetches atomic.Int32
	var engaged atomic.Bool
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  10 * time.Millisecond,
		Threshold: 20 * time.Millisecond,
		Fetch: func(ctx context.Context) error {
			fetches.Add(1)
			return nil
		},
		OnEngage:    func() { engaged.Store(true) },
		OnDisengage: func() { engaged.Store(false) },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = p.Run(ctx)
		close(done)
	}()

	p.SetSSEConnected(false)

	waitFor(t, 1*time.Second, func() bool { return fetches.Load() >= 2 },
		"poller never engaged and fetched after threshold")

	if !p.Active() {
		t.Errorf("expected Active=true while engaged")
	}
	if !engaged.Load() {
		t.Errorf("expected OnEngage callback to fire")
	}

	cancel()
	<-done
}

func TestFallbackPollerCancelledByReconnectBeforeThreshold(t *testing.T) {
	var fetches atomic.Int32
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  5 * time.Millisecond,
		Threshold: 100 * time.Millisecond,
		Fetch: func(ctx context.Context) error {
			fetches.Add(1)
			return nil
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = p.Run(ctx)
		close(done)
	}()

	p.SetSSEConnected(false)
	time.Sleep(20 * time.Millisecond) // well below 100ms threshold
	p.SetSSEConnected(true)
	time.Sleep(150 * time.Millisecond) // past original threshold

	if got := fetches.Load(); got != 0 {
		t.Errorf("expected 0 fetches when reconnect beats threshold, got %d", got)
	}
	if p.Active() {
		t.Errorf("expected Active=false; poller should never have engaged")
	}

	cancel()
	<-done
}

func TestFallbackPollerDisengagesOnReconnect(t *testing.T) {
	var fetches atomic.Int32
	var disengageCount atomic.Int32
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  5 * time.Millisecond,
		Threshold: 5 * time.Millisecond,
		Fetch: func(ctx context.Context) error {
			fetches.Add(1)
			return nil
		},
		OnDisengage: func() { disengageCount.Add(1) },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = p.Run(ctx)
		close(done)
	}()

	p.SetSSEConnected(false)
	waitFor(t, 500*time.Millisecond, p.Active, "poller never engaged")

	atFromEngage := fetches.Load()
	p.SetSSEConnected(true)

	waitFor(t, 500*time.Millisecond, func() bool { return !p.Active() },
		"poller never disengaged after reconnect")

	if got := disengageCount.Load(); got != 1 {
		t.Errorf("expected exactly 1 disengage callback, got %d", got)
	}

	// After disengaging, no more fetches should accumulate.
	time.Sleep(50 * time.Millisecond)
	if got := fetches.Load(); got > atFromEngage+1 {
		// allow 1 in-flight tick that races with disengage
		t.Errorf("fetches kept growing after disengage: had %d, now %d", atFromEngage, got)
	}

	cancel()
	<-done
}

func TestFallbackPollerStopsOnContextCancel(t *testing.T) {
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  5 * time.Millisecond,
		Threshold: 5 * time.Millisecond,
		Fetch:     func(ctx context.Context) error { return nil },
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = p.Run(ctx)
		close(done)
	}()

	p.SetSSEConnected(false)
	waitFor(t, 200*time.Millisecond, p.Active, "poller never engaged")

	cancel()
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatalf("poller did not exit on ctx cancel")
	}
}

// Race-y but useful: rapid flap should not deadlock.
func TestFallbackPollerHandlesRapidStateFlapping(t *testing.T) {
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  1 * time.Millisecond,
		Threshold: 1 * time.Millisecond,
		Fetch:     func(ctx context.Context) error { return nil },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = p.Run(ctx)
		close(done)
	}()

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				p.SetSSEConnected(j%2 == 0)
			}
		}()
	}
	wg.Wait()
	time.Sleep(20 * time.Millisecond) // settle

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("poller wedged after rapid flapping")
	}
}

// Fetch errors must not crash the poller; subsequent ticks must keep firing.
func TestFallbackPollerSurvivesFetchErrors(t *testing.T) {
	var fetches atomic.Int32
	p := newFallbackPoller(fallbackPollerConfig{
		Interval:  5 * time.Millisecond,
		Threshold: 5 * time.Millisecond,
		Fetch: func(ctx context.Context) error {
			fetches.Add(1)
			return errors.New("simulated")
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		_ = p.Run(ctx)
		close(done)
	}()

	p.SetSSEConnected(false)
	waitFor(t, 500*time.Millisecond, func() bool { return fetches.Load() >= 3 },
		"poller stopped fetching after error")

	cancel()
	<-done
}
