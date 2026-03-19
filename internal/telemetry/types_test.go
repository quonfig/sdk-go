package telemetry

import (
	"encoding/json"
	"testing"
)

func TestTelemetryEvents_JSONFormat(t *testing.T) {
	payload := TelemetryEvents{
		InstanceHash: "uuid-of-sdk-instance",
		Events: []TelemetryEvent{
			{
				Summaries: &EvalSummaries{
					Start: 1710000000,
					End:   1710000060,
					Summaries: []EvalSummary{
						{
							Key:  "new-checkout-flow",
							Type: "FEATURE_FLAG",
							Counters: []EvalCounter{
								{
									ConfigID:              "17605523587903695",
									ConditionalValueIndex: 1,
									ConfigRowIndex:        0,
									SelectedValue:         json.RawMessage(`{"bool":true}`),
									Count:                 4521,
									Reason:                0,
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Roundtrip
	var decoded TelemetryEvents
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.InstanceHash != "uuid-of-sdk-instance" {
		t.Errorf("instanceHash mismatch: %q", decoded.InstanceHash)
	}
	if len(decoded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(decoded.Events))
	}
	if decoded.Events[0].Summaries == nil {
		t.Fatal("expected summaries")
	}
	if decoded.Events[0].Summaries.Start != 1710000000 {
		t.Errorf("start mismatch: %d", decoded.Events[0].Summaries.Start)
	}
	if decoded.Events[0].Summaries.Summaries[0].Key != "new-checkout-flow" {
		t.Errorf("key mismatch: %q", decoded.Events[0].Summaries.Summaries[0].Key)
	}
	if decoded.Events[0].Summaries.Summaries[0].Counters[0].Count != 4521 {
		t.Errorf("count mismatch: %d", decoded.Events[0].Summaries.Summaries[0].Counters[0].Count)
	}
}

func TestTypedContextValue_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string", "hello", `{"string":"hello"}`},
		{"bool", true, `{"bool":true}`},
		{"int", int64(42), `{"int":42}`},
		{"double", 3.14, `{"double":3.14}`},
		{"stringList", []string{"a", "b"}, `{"stringList":["a","b"]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tv := NewTypedContextValue(tt.value)
			data, err := json.Marshal(tv)
			if err != nil {
				t.Fatalf("marshal failed: %v", err)
			}
			if string(data) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, string(data))
			}
		})
	}
}

func TestContextShapes_JSONFormat(t *testing.T) {
	payload := TelemetryEvents{
		InstanceHash: "uuid-of-sdk-instance",
		Events: []TelemetryEvent{
			{
				ContextShapes: &ContextShapes{
					Shapes: []ContextShape{
						{
							Name:       "user",
							FieldTypes: map[string]int{"key": 2, "email": 2, "activated": 5, "age": 1},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded TelemetryEvents
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	shapes := decoded.Events[0].ContextShapes.Shapes
	if len(shapes) != 1 {
		t.Fatalf("expected 1 shape, got %d", len(shapes))
	}
	if shapes[0].Name != "user" {
		t.Errorf("expected name 'user', got %q", shapes[0].Name)
	}
	if shapes[0].FieldTypes["age"] != FieldTypeInt {
		t.Errorf("expected age type %d, got %d", FieldTypeInt, shapes[0].FieldTypes["age"])
	}
}

func TestExampleContexts_JSONFormat(t *testing.T) {
	payload := TelemetryEvents{
		InstanceHash: "uuid-of-sdk-instance",
		Events: []TelemetryEvent{
			{
				ExampleContexts: &ExampleContextList{
					Examples: []ExampleContext{
						{
							Timestamp: 1710000000000,
							ContextSet: ExampleContextSet{
								Contexts: []NamedContextData{
									{
										Type: "user",
										Values: map[string]TypedContextValue{
											"key":   NewTypedContextValue("user-123"),
											"email": NewTypedContextValue("alice@acme.com"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify the JSON contains expected structure
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	events := raw["events"].([]interface{})
	event := events[0].(map[string]interface{})
	examples := event["exampleContexts"].(map[string]interface{})["examples"].([]interface{})
	example := examples[0].(map[string]interface{})

	if example["timestamp"].(float64) != 1710000000000 {
		t.Errorf("timestamp mismatch")
	}

	contexts := example["contextSet"].(map[string]interface{})["contexts"].([]interface{})
	ctx := contexts[0].(map[string]interface{})
	if ctx["type"].(string) != "user" {
		t.Errorf("type mismatch")
	}

	values := ctx["values"].(map[string]interface{})
	keyVal := values["key"].(map[string]interface{})
	if keyVal["string"].(string) != "user-123" {
		t.Errorf("key value mismatch")
	}
}
