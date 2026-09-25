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
//
// The number of distinct (context name, field) pairs per window is capped
// (P6): once reached, a NEW pair is dropped; existing pairs still update.
type ContextShapeAggregator struct {
	mu        sync.Mutex
	shapes    map[string]map[string]int // context name -> field name -> field type
	fields    int
	maxFields int
}

// NewContextShapeAggregator creates a new aggregator with the default cap
// (DefaultMaxContextShapeFields fields per window).
func NewContextShapeAggregator() *ContextShapeAggregator {
	return NewContextShapeAggregatorWithCap(DefaultMaxContextShapeFields)
}

// NewContextShapeAggregatorWithCap creates a new aggregator holding at most
// maxFields (context name, field) pairs per window (<= 0 means the default).
func NewContextShapeAggregatorWithCap(maxFields int) *ContextShapeAggregator {
	return &ContextShapeAggregator{
		shapes:    make(map[string]map[string]int),
		maxFields: intOr(maxFields, DefaultMaxContextShapeFields),
	}
}

// Record observes a context set and records its field types.
func (a *ContextShapeAggregator) Record(ctx ContextData) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for name, props := range ctx.Contexts {
		fields, ok := a.shapes[name]
		if !ok && a.fields < a.maxFields {
			fields = make(map[string]int)
			a.shapes[name] = fields
		}
		for field, value := range props {
			if _, ok := fields[field]; !ok {
				if a.fields >= a.maxFields {
					continue // cap reached: drop the new field (P6)
				}
				if fields == nil {
					fields = make(map[string]int)
					a.shapes[name] = fields
				}
				a.fields++
			}
			fields[field] = inferFieldType(value)
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
	a.fields = 0
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
