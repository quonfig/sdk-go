package telemetry

import "encoding/json"

// TelemetryEvents is the top-level envelope sent to POST /api/v1/telemetry/.
type TelemetryEvents struct {
	InstanceHash string           `json:"instanceHash"`
	Events       []TelemetryEvent `json:"events"`
}

// TelemetryEvent is a single event in the envelope. Exactly one field is set.
type TelemetryEvent struct {
	Summaries       *EvalSummaries      `json:"summaries,omitempty"`
	ContextShapes   *ContextShapes      `json:"contextShapes,omitempty"`
	ExampleContexts *ExampleContextList `json:"exampleContexts,omitempty"`
}

// --- Evaluation Summaries ---

// EvalSummaries is a time-windowed batch of evaluation counters.
type EvalSummaries struct {
	Start     int64         `json:"start"`
	End       int64         `json:"end"`
	Summaries []EvalSummary `json:"summaries"`
}

// EvalSummary groups evaluation counters for a single config key.
type EvalSummary struct {
	Key      string        `json:"key"`
	Type     string        `json:"type"`
	Counters []EvalCounter `json:"counters"`
}

// EvalCounter tracks how many times a specific evaluation outcome occurred.
type EvalCounter struct {
	ConfigID              string          `json:"configId"`
	ConditionalValueIndex int             `json:"conditionalValueIndex"`
	ConfigRowIndex        int             `json:"configRowIndex"`
	WeightedValueIndex    int             `json:"weightedValueIndex,omitempty"`
	SelectedValue         json.RawMessage `json:"selectedValue"`
	Count                 int64           `json:"count"`
	Reason                int             `json:"reason"`
}

// --- Context Shapes ---

// ContextShapes is a set of context type schemas.
type ContextShapes struct {
	Shapes []ContextShape `json:"shapes"`
}

// ContextShape describes the field types for a named context.
type ContextShape struct {
	Name       string         `json:"name"`
	FieldTypes map[string]int `json:"fieldTypes"`
}

// Field type codes matching the spec.
const (
	FieldTypeInt    = 1
	FieldTypeString = 2
	FieldTypeDouble = 4
	FieldTypeBool   = 5
	FieldTypeArray  = 10
)

// --- Example Contexts ---

// ExampleContextList holds sampled example contexts.
type ExampleContextList struct {
	Examples []ExampleContext `json:"examples"`
}

// ExampleContext is a single sampled context snapshot.
type ExampleContext struct {
	Timestamp  int64             `json:"timestamp"`
	ContextSet ExampleContextSet `json:"contextSet"`
}

// ExampleContextSet holds named contexts for serialization.
type ExampleContextSet struct {
	Contexts []NamedContextData `json:"contexts"`
}

// NamedContextData is a single named context with its properties.
type NamedContextData struct {
	Type   string                       `json:"type"`
	Values map[string]TypedContextValue `json:"values"`
}

// TypedContextValue wraps a context value with its type tag for JSON.
type TypedContextValue struct {
	value interface{}
}

func NewTypedContextValue(v interface{}) TypedContextValue {
	return TypedContextValue{value: v}
}

func (tv TypedContextValue) MarshalJSON() ([]byte, error) {
	switch v := tv.value.(type) {
	case bool:
		return json.Marshal(map[string]interface{}{"bool": v})
	case int, int32, int64:
		return json.Marshal(map[string]interface{}{"int": v})
	case float32, float64:
		return json.Marshal(map[string]interface{}{"double": v})
	case string:
		return json.Marshal(map[string]interface{}{"string": v})
	case []string:
		return json.Marshal(map[string]interface{}{"stringList": v})
	default:
		return json.Marshal(map[string]interface{}{"string": v})
	}
}
