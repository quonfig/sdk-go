package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ExampleContextAggregator stores one example of each unique context combination.
type ExampleContextAggregator struct {
	mu       sync.Mutex
	examples map[string]ExampleContext // grouped key -> example
}

// NewExampleContextAggregator creates a new aggregator.
func NewExampleContextAggregator() *ExampleContextAggregator {
	return &ExampleContextAggregator{
		examples: make(map[string]ExampleContext),
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
		values := make(map[string]TypedContextValue, len(props))
		for k, v := range props {
			values[k] = NewTypedContextValue(v)
		}
		contexts = append(contexts, NamedContextData{
			Type:   name,
			Values: values,
		})
	}
	return ExampleContextSet{Contexts: contexts}
}
