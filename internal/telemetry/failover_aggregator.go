package telemetry

import (
	"sync"
	"time"
)

// FailoverAggregator accumulates failover-behavior counters over a flush
// window: how many times the config-fetch hedge fired its secondary leg, how
// many installs the reject-older ordering guard dropped, and which upstream leg
// resolved each successful HTTP install. Every counter is additive and carries
// no user data. The aggregator is independently thread-safe (its own mutex) and
// is written directly from the failover call sites rather than through the
// telemetry queue — the call rate is per-config-refresh, not per-evaluation, so
// a plain mutex has negligible overhead.
type FailoverAggregator struct {
	mu                    sync.Mutex
	start                 int64 // unix millis; set on the first record of a window
	hedgeFired            int64
	guardRejected         int64
	resolvedFromPrimary   int64
	resolvedFromSecondary int64
	resolvedFromLkg       int64
}

// NewFailoverAggregator creates a new aggregator.
func NewFailoverAggregator() *FailoverAggregator {
	return &FailoverAggregator{}
}

// ensureStart stamps the window start on the first record. Caller holds mu.
func (a *FailoverAggregator) ensureStart() {
	if a.start == 0 {
		a.start = time.Now().UnixMilli()
	}
}

// RecordHedgeFired counts one config-fetch cycle whose hedge fired the
// secondary leg (the primary was slow or errored).
func (a *FailoverAggregator) RecordHedgeFired() {
	a.mu.Lock()
	a.ensureStart()
	a.hedgeFired++
	a.mu.Unlock()
}

// RecordGuardRejected counts one install dropped by the reject-older ordering
// guard (an equal-or-older snapshot on any install path, HTTP or SSE).
func (a *FailoverAggregator) RecordGuardRejected() {
	a.mu.Lock()
	a.ensureStart()
	a.guardRejected++
	a.mu.Unlock()
}

// RecordResolvedFrom counts one successful HTTP install by the leg that served
// it: sourceIndex 0 is the primary, any index > 0 is a failover/secondary leg.
// A negative index (SSE/datadir install with no HTTP leg) is ignored.
func (a *FailoverAggregator) RecordResolvedFrom(sourceIndex int) {
	if sourceIndex < 0 {
		return
	}
	a.mu.Lock()
	a.ensureStart()
	if sourceIndex == 0 {
		a.resolvedFromPrimary++
	} else {
		a.resolvedFromSecondary++
	}
	a.mu.Unlock()
}

// GetAndClear returns the window's counters as a TelemetryEvent and resets
// state. Returns nil if no failover activity occurred (every counter zero), so
// a healthy steady-state client emits no failover event at all.
func (a *FailoverAggregator) GetAndClear() *TelemetryEvent {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.hedgeFired == 0 && a.guardRejected == 0 &&
		a.resolvedFromPrimary == 0 && a.resolvedFromSecondary == 0 && a.resolvedFromLkg == 0 {
		return nil
	}

	event := &TelemetryEvent{
		Failover: &FailoverEvent{
			Start:                 a.start,
			End:                   time.Now().UnixMilli(),
			HedgeFired:            a.hedgeFired,
			GuardRejected:         a.guardRejected,
			ResolvedFromPrimary:   a.resolvedFromPrimary,
			ResolvedFromSecondary: a.resolvedFromSecondary,
			ResolvedFromLkg:       a.resolvedFromLkg,
		},
	}

	a.start = 0
	a.hedgeFired = 0
	a.guardRejected = 0
	a.resolvedFromPrimary = 0
	a.resolvedFromSecondary = 0
	a.resolvedFromLkg = 0

	return event
}
