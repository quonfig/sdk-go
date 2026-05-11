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

// TestExampleContexts_JSONFormat locks in the wire shape that every other
// Quonfig SDK emits: values are a flat map, NOT wrapped in {"<type>": v}.
// Previously sdk-go wrapped each value, which broke the ClickHouse MV's
// JSONExtractString(values, 'key') unwrap and rendered properties as
// "[object Object]" in the search-context UI.
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
										Values: map[string]interface{}{
											"key":   "user-123",
											"email": "alice@acme.com",
											"age":   30,
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

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

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

	// String value must be a plain string, not {"string": "..."}.
	if got, ok := values["key"].(string); !ok || got != "user-123" {
		t.Errorf("values.key: expected plain string \"user-123\", got %T %v", values["key"], values["key"])
	}
	if got, ok := values["email"].(string); !ok || got != "alice@acme.com" {
		t.Errorf("values.email: expected plain string, got %T %v", values["email"], values["email"])
	}
	// Int value must be a number, not {"int": N}.
	if got, ok := values["age"].(float64); !ok || got != 30 {
		t.Errorf("values.age: expected numeric 30, got %T %v", values["age"], values["age"])
	}

	// Belt-and-suspenders: assert the literal JSON bytes do NOT contain
	// type-tag wrappers anywhere in the values block. If anyone adds a
	// MarshalJSON that re-introduces wrapping, this catches it.
	jsonStr := string(data)
	for _, wrapperKey := range []string{`{"string":`, `{"int":`, `{"bool":`, `{"double":`, `{"stringList":`} {
		if containsInValues(jsonStr, wrapperKey) {
			t.Errorf("wire format regressed to wrapped values: found %q in JSON %s", wrapperKey, jsonStr)
		}
	}
}

// containsInValues reports whether needle appears in the JSON output inside
// the "values":{...} block of an example context. We do a coarse substring
// check — the test data has no other place a {"string": ... pattern could
// legitimately appear, so any match means a regression.
func containsInValues(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
