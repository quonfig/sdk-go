package telemetry

import (
	"testing"
)

func TestContextShapeAggregator_Empty(t *testing.T) {
	agg := NewContextShapeAggregator()
	event := agg.GetAndClear()
	if event != nil {
		t.Fatal("expected nil event for empty aggregator")
	}
}

func TestContextShapeAggregator_RecordsFieldTypes(t *testing.T) {
	agg := NewContextShapeAggregator()

	agg.Record(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {
				"key":       "user-123",
				"email":     "alice@example.com",
				"age":       42,
				"activated": true,
				"score":     3.14,
			},
		},
	})

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.ContextShapes == nil {
		t.Fatal("expected context shapes")
	}
	if len(event.ContextShapes.Shapes) != 1 {
		t.Fatalf("expected 1 shape, got %d", len(event.ContextShapes.Shapes))
	}

	shape := event.ContextShapes.Shapes[0]
	if shape.Name != "user" {
		t.Errorf("expected name 'user', got %q", shape.Name)
	}

	tests := map[string]int{
		"key":       FieldTypeString,
		"email":     FieldTypeString,
		"age":       FieldTypeInt,
		"activated": FieldTypeBool,
		"score":     FieldTypeDouble,
	}
	for field, expectedType := range tests {
		if got, ok := shape.FieldTypes[field]; !ok {
			t.Errorf("missing field %q", field)
		} else if got != expectedType {
			t.Errorf("field %q: expected type %d, got %d", field, expectedType, got)
		}
	}
}

func TestContextShapeAggregator_MergesAcrossRecords(t *testing.T) {
	agg := NewContextShapeAggregator()

	agg.Record(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {"key": "u1"},
		},
	})
	agg.Record(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {"email": "a@b.com"},
			"org":  {"name": "Acme"},
		},
	})

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if len(event.ContextShapes.Shapes) != 2 {
		t.Fatalf("expected 2 shapes (user, org), got %d", len(event.ContextShapes.Shapes))
	}

	// Find user shape
	for _, shape := range event.ContextShapes.Shapes {
		if shape.Name == "user" {
			if len(shape.FieldTypes) != 2 {
				t.Errorf("expected 2 fields for user (merged), got %d", len(shape.FieldTypes))
			}
		}
	}
}

func TestContextShapeAggregator_ClearsAfterGet(t *testing.T) {
	agg := NewContextShapeAggregator()

	agg.Record(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {"key": "u1"},
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
