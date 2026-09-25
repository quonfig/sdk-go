package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ExampleContextAggregator stores one example of each unique context
// combination, at most maxExamples per window (P6: drop newest when full).
type ExampleContextAggregator struct {
	mu          sync.Mutex
	examples    map[string]ExampleContext // grouped key -> example
	maxExamples int
}

// NewExampleContextAggregator creates a new aggregator with the default cap
// (DefaultMaxExampleContexts examples per window).
func NewExampleContextAggregator() *ExampleContextAggregator {
	return NewExampleContextAggregatorWithCap(DefaultMaxExampleContexts)
}

// NewExampleContextAggregatorWithCap creates a new aggregator holding at most
// maxExamples examples per window (<= 0 means the default).
func NewExampleContextAggregatorWithCap(maxExamples int) *ExampleContextAggregator {
	return &ExampleContextAggregator{
		examples:    make(map[string]ExampleContext),
		maxExamples: intOr(maxExamples, DefaultMaxExampleContexts),
	}
}

// Record stores a context example, deduplicating by grouped key.
func (a *ExampleContextAggregator) Record(ctx ContextData) {
	key := contextGroupKey(ctx)

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.examples[key]; ok {
		return // already have this combination
	}
	if len(a.examples) >= a.maxExamples {
		return // cap reached: drop the new example (P6)
	}

	a.examples[key] = ExampleContext{
		Timestamp:  time.Now().UnixMilli(),
		ContextSet: contextDataToExampleContextSet(ctx),
	}
}

// GetAndClear returns the current examples and resets state. Returns nil if empty.
func (a *ExampleContextAggregator) GetAndClear() *TelemetryEvent {
	a.mu.Lock()
	defer a.mu.Unlock()

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
