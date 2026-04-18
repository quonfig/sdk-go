package evalcore

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestEvalcoreValueUnmarshalJSONObject(t *testing.T) {
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

func TestEvalcoreValueUnmarshalJSONRejectsString(t *testing.T) {
	raw := `{"type":"json","value":"{\"a\":1}"}`
	var v Value
	err := json.Unmarshal([]byte(raw), &v)
	if err == nil {
		t.Fatal("expected error for stringified json, got nil")
	}
}
