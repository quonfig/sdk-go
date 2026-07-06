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
	Failover        *FailoverEvent      `json:"failover,omitempty"`
}

// --- Failover observability ---

// FailoverEvent carries per-flush-window failover counters. It is additive on
// the wire (an older api-telemetry strips the unknown field) and is only sent
// when at least one counter is non-zero, so a healthy client emits nothing.
// Start/End are unix millis, matching the eval-summary window convention.
type FailoverEvent struct {
	Start                 int64 `json:"start"`
	End                   int64 `json:"end"`
	HedgeFired            int64 `json:"hedgeFired"`
	GuardRejected         int64 `json:"guardRejected"`
	ResolvedFromPrimary   int64 `json:"resolvedFromPrimary"`
	ResolvedFromSecondary int64 `json:"resolvedFromSecondary"`
	ResolvedFromLkg       int64 `json:"resolvedFromLkg"`
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
// Values are emitted as a flat JSON map (e.g. {"key":"user-123","age":30}),
// matching every other SDK's wire format. They are NOT wrapped in a type tag.
type NamedContextData struct {
	Type   string                 `json:"type"`
	Values map[string]interface{} `json:"values"`
}
