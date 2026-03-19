package telemetry

import (
	"testing"
)

func TestExampleContextAggregator_Empty(t *testing.T) {
	agg := NewExampleContextAggregator()
	event := agg.GetAndClear()
	if event != nil {
		t.Fatal("expected nil event for empty aggregator")
	}
}

func TestExampleContextAggregator_StoresExample(t *testing.T) {
	agg := NewExampleContextAggregator()

	agg.Record(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {"key": "user-123", "email": "alice@example.com"},
		},
	})

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.ExampleContexts == nil {
		t.Fatal("expected example contexts")
	}
	if len(event.ExampleContexts.Examples) != 1 {
		t.Fatalf("expected 1 example, got %d", len(event.ExampleContexts.Examples))
	}

	example := event.ExampleContexts.Examples[0]
	if example.Timestamp == 0 {
		t.Error("expected non-zero timestamp")
	}
	if len(example.ContextSet.Contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(example.ContextSet.Contexts))
	}
	if example.ContextSet.Contexts[0].Type != "user" {
		t.Errorf("expected type 'user', got %q", example.ContextSet.Contexts[0].Type)
	}
}

func TestExampleContextAggregator_DeduplicatesByKey(t *testing.T) {
	agg := NewExampleContextAggregator()

	// Same user key -> should deduplicate
	for i := 0; i < 10; i++ {
		agg.Record(ContextData{
			Contexts: map[string]map[string]interface{}{
				"user": {"key": "user-123"},
			},
		})
	}

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if len(event.ExampleContexts.Examples) != 1 {
		t.Errorf("expected 1 example (deduplicated), got %d", len(event.ExampleContexts.Examples))
	}
}

func TestExampleContextAggregator_DifferentKeysStored(t *testing.T) {
	agg := NewExampleContextAggregator()

	agg.Record(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {"key": "user-1"},
		},
	})
	agg.Record(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {"key": "user-2"},
		},
	})

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if len(event.ExampleContexts.Examples) != 2 {
		t.Errorf("expected 2 examples (different keys), got %d", len(event.ExampleContexts.Examples))
	}
}

func TestExampleContextAggregator_ClearsAfterGet(t *testing.T) {
	agg := NewExampleContextAggregator()

	agg.Record(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {"key": "user-1"},
		},
	})

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}

	event2 := agg.GetAndClear()
	if event2 != nil {
		t.Fatal("expected nil after clear")
	}
}
