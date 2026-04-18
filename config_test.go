package quonfig

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestValueUnmarshalBool(t *testing.T) {
	raw := `{"type":"bool","value":true}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.Type != ValueTypeBool {
		t.Errorf("expected type bool, got %s", v.Type)
	}
	if !v.BoolValue() {
		t.Error("expected true")
	}
}

func TestValueUnmarshalInt(t *testing.T) {
	raw := `{"type":"int","value":42}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.IntValue() != 42 {
		t.Errorf("expected 42, got %d", v.IntValue())
	}
}

func TestValueUnmarshalIntFromString(t *testing.T) {
	raw := `{"type":"int","value":"99"}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.IntValue() != 99 {
		t.Errorf("expected 99, got %d", v.IntValue())
	}
}

func TestValueUnmarshalDouble(t *testing.T) {
	raw := `{"type":"double","value":3.14}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.DoubleValue() != 3.14 {
		t.Errorf("expected 3.14, got %f", v.DoubleValue())
	}
}

func TestValueUnmarshalString(t *testing.T) {
	raw := `{"type":"string","value":"hello"}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.StringValue() != "hello" {
		t.Errorf("expected hello, got %s", v.StringValue())
	}
}

func TestValueUnmarshalJSONObject(t *testing.T) {
	raw := `{"type":"json","value":{"a":1,"b":"c"}}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.Type != ValueTypeJSON {
		t.Errorf("expected type json, got %s", v.Type)
	}
	m, ok := v.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", v.Value)
	}
	if fmt.Sprintf("%v", m["a"]) != "1" {
		t.Errorf("expected a=1, got %v", m["a"])
	}
	if m["b"] != "c" {
		t.Errorf("expected b=c, got %v", m["b"])
	}
}

func TestValueUnmarshalJSONArray(t *testing.T) {
	raw := `{"type":"json","value":[1,2,3]}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	arr, ok := v.Value.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", v.Value)
	}
	if len(arr) != 3 {
		t.Errorf("expected 3 elements, got %d", len(arr))
	}
}

func TestValueUnmarshalJSONRejectsString(t *testing.T) {
	raw := `{"type":"json","value":"{\"a\":1}"}`
	var v Value
	err := json.Unmarshal([]byte(raw), &v)
	if err == nil {
		t.Fatal("expected error for stringified json, got nil")
	}
}

func TestValueUnmarshalStringList(t *testing.T) {
	raw := `{"type":"string_list","value":["a","b","c"]}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	sl := v.StringListValue()
	if len(sl) != 3 || sl[0] != "a" || sl[1] != "b" || sl[2] != "c" {
		t.Errorf("expected [a b c], got %v", sl)
	}
}

func TestValueUnmarshalWeightedValues(t *testing.T) {
	raw := `{"type":"weighted_values","value":{"weightedValues":[{"weight":90,"value":{"type":"string","value":"control"}},{"weight":10,"value":{"type":"string","value":"experiment"}}],"hashByPropertyName":"user.key"}}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	wv := v.WeightedValuesValue()
	if wv == nil {
		t.Fatal("expected weighted values")
	}
	if len(wv.WeightedValues) != 2 {
		t.Errorf("expected 2 weighted values, got %d", len(wv.WeightedValues))
	}
	if wv.HashByPropertyName != "user.key" {
		t.Errorf("expected hashByPropertyName=user.key, got %s", wv.HashByPropertyName)
	}
}

func TestValueUnmarshalConfidential(t *testing.T) {
	raw := `{"type":"string","value":"secret","confidential":true,"decryptWith":"key-123"}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if !v.Confidential {
		t.Error("expected confidential=true")
	}
	if v.DecryptWith != "key-123" {
		t.Errorf("expected decryptWith=key-123, got %s", v.DecryptWith)
	}
}

func TestValueUnmarshalProvided(t *testing.T) {
	raw := `{"type":"provided","value":{"source":"ENV_VAR","lookup":"MY_SECRET"}}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.Type != ValueTypeProvided {
		t.Errorf("expected type provided, got %s", v.Type)
	}
	pd := v.ProvidedValue()
	if pd == nil {
		t.Fatal("expected ProvidedData, got nil")
	}
	if pd.Source != "ENV_VAR" {
		t.Errorf("expected source ENV_VAR, got %s", pd.Source)
	}
	if pd.Lookup != "MY_SECRET" {
		t.Errorf("expected lookup MY_SECRET, got %s", pd.Lookup)
	}
}

func TestValueUnmarshalSchema(t *testing.T) {
	raw := `{"type":"schema","value":{"schemaType":"ZOD","schema":"z.string()"}}`
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.Type != ValueTypeSchema {
		t.Errorf("expected type schema, got %s", v.Type)
	}
	sd, ok := v.Value.(*SchemaData)
	if !ok || sd == nil {
		t.Fatal("expected SchemaData")
	}
	if sd.SchemaType != "ZOD" {
		t.Errorf("expected schemaType ZOD, got %s", sd.SchemaType)
	}
	if sd.Schema != "z.string()" {
		t.Errorf("expected schema z.string(), got %s", sd.Schema)
	}
}

func TestProvidedValueNil(t *testing.T) {
	v := Value{Type: ValueTypeString, Value: "hello"}
	if v.ProvidedValue() != nil {
		t.Error("expected nil ProvidedValue for string type")
	}
}

func TestConfigEnvelopeUnmarshal(t *testing.T) {
	raw := `{
		"configs": [
			{
				"id": "cfg-1",
				"key": "feature.enabled",
				"type": "feature_flag",
				"valueType": "bool",
				"sendToClientSdk": false,
				"default": {
					"rules": [
						{
							"criteria": [],
							"value": {"type": "bool", "value": true}
						}
					]
				}
			}
		],
		"meta": {
			"version": "v42",
			"environment": "production",
			"workspaceId": "ws-1"
		}
	}`

	var envelope ConfigEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(envelope.Configs))
	}
	cfg := envelope.Configs[0]
	if cfg.Key != "feature.enabled" {
		t.Errorf("expected key feature.enabled, got %s", cfg.Key)
	}
	if cfg.Type != ConfigTypeFeatureFlag {
		t.Errorf("expected type feature_flag, got %s", cfg.Type)
	}
	if !cfg.Default.Rules[0].Value.BoolValue() {
		t.Error("expected default value true")
	}
	if envelope.Meta.Version != "v42" {
		t.Errorf("expected version v42, got %s", envelope.Meta.Version)
	}
	if envelope.Meta.WorkspaceID != "ws-1" {
		t.Errorf("expected workspaceId ws-1, got %s", envelope.Meta.WorkspaceID)
	}
}
