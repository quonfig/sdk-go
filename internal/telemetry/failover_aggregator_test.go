package telemetry

import "testing"

func TestFailoverAggregator_EmptyReturnsNil(t *testing.T) {
	a := NewFailoverAggregator()
	if got := a.GetAndClear(); got != nil {
		t.Fatalf("expected nil when no failover activity recorded, got %+v", got)
	}
}

func TestFailoverAggregator_CountsAndClears(t *testing.T) {
	a := NewFailoverAggregator()
	a.RecordHedgeFired()
	a.RecordHedgeFired()
	a.RecordGuardRejected()
	a.RecordResolvedFrom(0) // primary
	a.RecordResolvedFrom(1) // secondary
	a.RecordResolvedFrom(2) // secondary (any index > 0)

	event := a.GetAndClear()
	if event == nil || event.Failover == nil {
		t.Fatal("expected a failover event")
	}
	f := event.Failover
	if f.HedgeFired != 2 {
		t.Errorf("HedgeFired = %d, want 2", f.HedgeFired)
	}
	if f.GuardRejected != 1 {
		t.Errorf("GuardRejected = %d, want 1", f.GuardRejected)
	}
	if f.ResolvedFromPrimary != 1 {
		t.Errorf("ResolvedFromPrimary = %d, want 1", f.ResolvedFromPrimary)
	}
	if f.ResolvedFromSecondary != 2 {
		t.Errorf("ResolvedFromSecondary = %d, want 2", f.ResolvedFromSecondary)
	}
	if f.ResolvedFromLkg != 0 {
		t.Errorf("ResolvedFromLkg = %d, want 0", f.ResolvedFromLkg)
	}
	if f.Start <= 0 || f.End < f.Start {
		t.Errorf("expected a sane window [start=%d, end=%d]", f.Start, f.End)
	}

	// After GetAndClear the aggregator resets: a subsequent empty window is nil.
	if got := a.GetAndClear(); got != nil {
		t.Fatalf("expected nil after clear, got %+v", got)
	}
}

func TestFailoverAggregator_NegativeSourceIndexIgnored(t *testing.T) {
	a := NewFailoverAggregator()
	// sourceIndex -1 (SSE/datadir install with no HTTP leg) must not count as a
	// resolved-from and must not, by itself, produce an event.
	a.RecordResolvedFrom(-1)
	if got := a.GetAndClear(); got != nil {
		t.Fatalf("expected nil when only a no-leg install was recorded, got %+v", got)
	}
}
