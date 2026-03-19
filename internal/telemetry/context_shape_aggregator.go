package telemetry

import (
	"sync"
)

// ContextData is the telemetry package's view of a context set.
type ContextData struct {
	// Contexts maps context name -> property name -> property value.
	Contexts map[string]map[string]interface{}
}

// ContextShapeAggregator tracks the field types seen in evaluation contexts.
type ContextShapeAggregator struct {
	mu     sync.Mutex
	shapes map[string]map[string]int // context name -> field name -> field type
}

// NewContextShapeAggregator creates a new aggregator.
func NewContextShapeAggregator() *ContextShapeAggregator {
	return &ContextShapeAggregator{
		shapes: make(map[string]map[string]int),
	}
}

// Record observes a context set and records its field types.
func (a *ContextShapeAggregator) Record(ctx ContextData) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for name, props := range ctx.Contexts {
		if _, ok := a.shapes[name]; !ok {
			a.shapes[name] = make(map[string]int)
		}
		for field, value := range props {
			a.shapes[name][field] = inferFieldType(value)
		}
	}
}

// GetAndClear returns the current shapes and resets state. Returns nil if empty.
func (a *ContextShapeAggregator) GetAndClear() *TelemetryEvent {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.shapes) == 0 {
		return nil
	}

	shapes := make([]ContextShape, 0, len(a.shapes))
	for name, fields := range a.shapes {
		shapes = append(shapes, ContextShape{
			Name:       name,
			FieldTypes: fields,
		})
	}

	event := &TelemetryEvent{
		ContextShapes: &ContextShapes{
			Shapes: shapes,
		},
	}

	a.shapes = make(map[string]map[string]int)
	return event
}

// inferFieldType returns the telemetry field type code for a value.
func inferFieldType(v interface{}) int {
	switch v.(type) {
	case bool:
		return FieldTypeBool
	case int, int32, int64:
		return FieldTypeInt
	case float32, float64:
		return FieldTypeDouble
	case []string, []interface{}:
		return FieldTypeArray
	default:
		return FieldTypeString
	}
}
