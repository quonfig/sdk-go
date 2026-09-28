package telemetry

import (
	"fmt"
	"testing"
	"time"
)

// qfg-cg1e: each context-group key is sent at most once per hour (TTL 1h),
// through a seen-map capped at maxSeen entries. When the map is full a new
// key is dropped (drop newest) and not recorded, so it is sent on a later
// evaluation. Expired entries are pruned once per flush window, never per
// Record call.

type fakeNow struct{ t time.Time }

func (f *fakeNow) now() time.Time { return f.t }

func userCtx(key string) ContextData {
	return ContextData{Contexts: map[string]map[string]interface{}{
		"user": {"key": key, "email": key + "@x.test"},
	}}
}

func exampleCount(ev *TelemetryEvent) int {
	if ev == nil || ev.ExampleContexts == nil {
		return 0
	}
	return len(ev.ExampleContexts.Examples)
}

func TestExampleContextSentOncePerHour(t *testing.T) {
	clk := &fakeNow{t: time.Unix(1_000_000, 0)}
	a := newExampleContextAggregator(10, 100, clk.now)

	a.Record(userCtx("u1"))
	if n := exampleCount(a.GetAndClear()); n != 1 {
		t.Fatalf("window 1: %d examples, want 1", n)
	}

	clk.t = clk.t.Add(60 * time.Second)
	a.Record(userCtx("u1"))
	if n := exampleCount(a.GetAndClear()); n != 0 {
		t.Fatalf("window 2 (same context, <1h later): %d examples, want 0", n)
	}

	clk.t = clk.t.Add(time.Hour)
	a.Record(userCtx("u1"))
	if n := exampleCount(a.GetAndClear()); n != 1 {
		t.Fatalf("after 1h+: %d examples, want 1", n)
	}
}

func TestExampleContextSeenCapDropsNewestWithoutRecording(t *testing.T) {
	clk := &fakeNow{t: time.Unix(1_000_000, 0)}
	a := newExampleContextAggregator(10, 2, clk.now)

	a.Record(userCtx("a"))
	a.Record(userCtx("b"))
	a.Record(userCtx("c")) // seen-map full: dropped, not recorded
	if n := exampleCount(a.GetAndClear()); n != 2 {
		t.Fatalf("%d examples, want 2 (third dropped at seen cap)", n)
	}
	if _, ok := a.seen[contextGroupKey(userCtx("c"))]; ok {
		t.Fatalf("dropped context was written to the seen-map")
	}
	if len(a.seen) != 2 {
		t.Fatalf("seen-map size %d, want 2 (cap)", len(a.seen))
	}

	// Once a and b expire and a flush prunes them, c is recorded.
	clk.t = clk.t.Add(time.Hour + time.Second)
	_ = a.GetAndClear()
	a.Record(userCtx("c"))
	if n := exampleCount(a.GetAndClear()); n != 1 {
		t.Fatalf("after expiry: %d examples, want 1 (c deferred, not lost)", n)
	}
}

func TestExampleContextWindowCapSkipDoesNotRecordSeen(t *testing.T) {
	clk := &fakeNow{t: time.Unix(1_000_000, 0)}
	a := newExampleContextAggregator(1, 100, clk.now)

	a.Record(userCtx("a"))
	a.Record(userCtx("b")) // window full: skipped
	if n := exampleCount(a.GetAndClear()); n != 1 {
		t.Fatalf("%d examples, want 1", n)
	}
	a.Record(userCtx("b"))
	if n := exampleCount(a.GetAndClear()); n != 1 {
		t.Fatalf("next window: %d examples, want 1 (b deferred, not lost)", n)
	}
}

func TestExampleContextPruneNotRunPerRecord(t *testing.T) {
	clk := &fakeNow{t: time.Unix(1_000_000, 0)}
	a := newExampleContextAggregator(100_000, 10, clk.now)

	for i := 0; i < 10; i++ {
		a.Record(userCtx(fmt.Sprintf("fill-%d", i)))
	}
	// Map is full of fresh entries; many new keys arrive.
	for i := 0; i < 1000; i++ {
		a.Record(userCtx(fmt.Sprintf("new-%d", i)))
	}
	if a.pruneRuns != 0 {
		t.Fatalf("prune ran %d times during Record, want 0", a.pruneRuns)
	}
	_ = a.GetAndClear()
	if a.pruneRuns != 1 {
		t.Fatalf("prune ran %d times for one flush, want 1", a.pruneRuns)
	}
}
