package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ExampleContextSeenTTL is how long a context-group key stays in the
// rate-limit seen-map: each key is sent at most once per this interval, the
// same as the other server SDKs (qfg-cg1e).
const ExampleContextSeenTTL = time.Hour

// ExampleContextAggregator stores one example of each unique context
// combination, at most maxExamples per window (P6: drop newest when full).
//
// A seen-map rate-limits each context-group key to one example per
// ExampleContextSeenTTL across windows. It holds at most maxSeen keys; when
// it is full, a new key is dropped (drop newest) and not written to the map,
// so it is recorded on a later evaluation once room frees up. Expired keys
// are pruned once per flush (GetAndClear), never on the Record hot path.
type ExampleContextAggregator struct {
	mu          sync.Mutex
	examples    map[string]ExampleContext // grouped key -> example
	maxExamples int

	seen    map[string]time.Time // grouped key -> when last recorded
	maxSeen int
	now     func() time.Time

	pruneRuns int // test hook: number of prune passes
}

// NewExampleContextAggregator creates a new aggregator with the default caps
// (DefaultMaxExampleContexts examples per window, DefaultMaxExampleContextsSeen
// rate-limit keys).
func NewExampleContextAggregator() *ExampleContextAggregator {
	return NewExampleContextAggregatorWithCaps(DefaultMaxExampleContexts, DefaultMaxExampleContextsSeen)
}

// NewExampleContextAggregatorWithCap creates a new aggregator holding at most
// maxExamples examples per window (<= 0 means the default), with the default
// seen-map cap.
func NewExampleContextAggregatorWithCap(maxExamples int) *ExampleContextAggregator {
	return NewExampleContextAggregatorWithCaps(maxExamples, DefaultMaxExampleContextsSeen)
}

// NewExampleContextAggregatorWithCaps creates a new aggregator holding at
// most maxExamples examples per window and at most maxSeen keys in the
// rate-limit seen-map (<= 0 means the default for either).
func NewExampleContextAggregatorWithCaps(maxExamples, maxSeen int) *ExampleContextAggregator {
	return newExampleContextAggregator(maxExamples, maxSeen, time.Now)
}

func newExampleContextAggregator(maxExamples, maxSeen int, now func() time.Time) *ExampleContextAggregator {
	return &ExampleContextAggregator{
		examples:    make(map[string]ExampleContext),
		maxExamples: intOr(maxExamples, DefaultMaxExampleContexts),
		seen:        make(map[string]time.Time),
		maxSeen:     intOr(maxSeen, DefaultMaxExampleContextsSeen),
		now:         now,
	}
}

// Record stores a context example, deduplicating by grouped key within the
// window and rate-limiting each key to once per ExampleContextSeenTTL.
func (a *ExampleContextAggregator) Record(ctx ContextData) {
	if !hasIdentifyingKey(ctx) {
		return // no "key"/"trackingId" on any context: not reportable (matches sdk-node)
	}
	key := contextGroupKey(ctx)

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.examples[key]; ok {
		return // already have this combination
	}
	now := a.now()
	last, seen := a.seen[key]
	if seen && now.Sub(last) < ExampleContextSeenTTL {
		return // sent within the last hour
	}
	if len(a.examples) >= a.maxExamples {
		return // cap reached: drop the new example (P6); not marked seen
	}
	if !seen && len(a.seen) >= a.maxSeen {
		return // seen-map full: drop newest; not marked seen, so retried later
	}

	a.seen[key] = now
	a.examples[key] = ExampleContext{
		Timestamp:  now.UnixMilli(),
		ContextSet: contextDataToExampleContextSet(ctx),
	}
}

// GetAndClear returns the current examples and resets state. Returns nil if
// empty. It also prunes expired keys from the seen-map (once per flush).
func (a *ExampleContextAggregator) GetAndClear() *TelemetryEvent {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.pruneSeen()

	if len(a.examples) == 0 {
		return nil
	}

	examples := make([]ExampleContext, 0, len(a.examples))
	for _, ex := range a.examples {
		examples = append(examples, ex)
	}

	event := &TelemetryEvent{
		ExampleContexts: &ExampleContextList{
			Examples: examples,
		},
	}

	a.examples = make(map[string]ExampleContext)
	return event
}

// pruneSeen drops seen-map keys older than ExampleContextSeenTTL. Caller
// holds a.mu. Called once per flush only.
func (a *ExampleContextAggregator) pruneSeen() {
	a.pruneRuns++
	now := a.now()
	for k, t := range a.seen {
		if now.Sub(t) >= ExampleContextSeenTTL {
			delete(a.seen, k)
		}
	}
}

// hasIdentifyingKey reports whether any context in ctx carries a non-empty
// "key" or "trackingId" property. Example contexts without one are dropped.
func hasIdentifyingKey(ctx ContextData) bool {
	for _, props := range ctx.Contexts {
		for _, name := range [...]string{"key", "trackingId"} {
			if v, ok := props[name]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				return true
			}
		}
	}
	return false
}

// contextGroupKey produces a stable key for deduplication based on context names and their key values.
func contextGroupKey(ctx ContextData) string {
	parts := make([]string, 0, len(ctx.Contexts))
	for name, props := range ctx.Contexts {
		// Use the "key" property if present, otherwise use the context name alone
		if keyVal, ok := props["key"]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", name, keyVal))
		} else {
			parts = append(parts, name)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

// contextDataToExampleContextSet converts ContextData to the serializable format.
func contextDataToExampleContextSet(ctx ContextData) ExampleContextSet {
	contexts := make([]NamedContextData, 0, len(ctx.Contexts))
	for name, props := range ctx.Contexts {
		values := make(map[string]interface{}, len(props))
		for k, v := range props {
			values[k] = v
		}
		contexts = append(contexts, NamedContextData{
			Type:   name,
			Values: values,
		})
	}
	return ExampleContextSet{Contexts: contexts}
}
