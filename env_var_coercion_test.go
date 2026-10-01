package quonfig

import (
	"errors"
	"reflect"
	"testing"
)

// ENV_VAR-provided values coerce to the config's value type through the
// public getters (qfg-2agi.21): string_list splits on commas and trims each
// item; json parses; an uncoercible value returns ErrUnableToCoerce.
func envVarClient(t *testing.T, valueType ValueType, envValue string) *Client {
	t.Helper()
	return detailsClient(t, []ConfigResponse{{
		ID:        "cfg-env",
		Key:       "env.cfg",
		Type:      ConfigTypeConfig,
		ValueType: valueType,
		Default: RuleSet{Rules: []Rule{{
			Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
			Value:    Value{Type: ValueTypeProvided, Value: &ProvidedData{Source: "ENV_VAR", Lookup: "QF_ENV"}},
		}}},
	}}, func(key string) (string, bool) {
		if key == "QF_ENV" {
			return envValue, true
		}
		return "", false
	})
}

func TestEnvVar_StringListSplitsOnComma(t *testing.T) {
	cases := map[string][]string{
		"a,b,c":     {"a", "b", "c"},
		"a, b ,  c": {"a", "b", "c"},
		"single":    {"single"},
		"":          {},
		"a,,b":      {"a", "", "b"},
	}
	for env, want := range cases {
		got, ok, err := envVarClient(t, ValueTypeStringList, env).GetStringSliceValue("env.cfg", nil)
		if err != nil || !ok {
			t.Fatalf("GetStringSliceValue(env=%q) ok=%v err=%v", env, ok, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetStringSliceValue(env=%q) = %#v, want %#v", env, got, want)
		}
	}
}

func TestEnvVar_JSONParses(t *testing.T) {
	cases := map[string]interface{}{
		`{"a":1,"b":[true,"x"]}`: map[string]interface{}{"a": float64(1), "b": []interface{}{true, "x"}},
		`[1,2]`:                  []interface{}{float64(1), float64(2)},
		`42`:                     float64(42),
		`null`:                   nil,
	}
	for env, want := range cases {
		got, ok, err := envVarClient(t, ValueTypeJSON, env).GetJSONValue("env.cfg", nil)
		if err != nil || !ok {
			t.Fatalf("GetJSONValue(env=%q) ok=%v err=%v", env, ok, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("GetJSONValue(env=%q) = %#v, want %#v", env, got, want)
		}
	}
}

func TestEnvVar_UncoercibleReturnsErrUnableToCoerce(t *testing.T) {
	cases := []struct {
		valueType ValueType
		env       string
		get       func(c *Client) (bool, error)
	}{
		{ValueTypeJSON, `{"a":`, func(c *Client) (bool, error) { _, ok, err := c.GetJSONValue("env.cfg", nil); return ok, err }},
		{ValueTypeJSON, `not json`, func(c *Client) (bool, error) { _, ok, err := c.GetJSONValue("env.cfg", nil); return ok, err }},
		{ValueTypeInt, `30s`, func(c *Client) (bool, error) { _, ok, err := c.GetIntValue("env.cfg", nil); return ok, err }},
		{ValueTypeDouble, `abc`, func(c *Client) (bool, error) { _, ok, err := c.GetFloatValue("env.cfg", nil); return ok, err }},
		{ValueTypeBool, `maybe`, func(c *Client) (bool, error) { _, ok, err := c.GetBoolValue("env.cfg", nil); return ok, err }},
		{ValueTypeDuration, `30s`, func(c *Client) (bool, error) { _, ok, err := c.GetDurationValue("env.cfg", nil); return ok, err }},
		// Validated at coercion, so even the untyped string read of an env
		// duration fails the same way a typed read does.
		{ValueTypeDuration, `PT0.5H`, func(c *Client) (bool, error) { _, ok, err := c.GetStringValue("env.cfg", nil); return ok, err }},
	}
	for _, c := range cases {
		ok, err := c.get(envVarClient(t, c.valueType, c.env))
		if ok || !errors.Is(err, ErrUnableToCoerce) {
			t.Errorf("%s env=%q: ok=%v err=%v, want ok=false and ErrUnableToCoerce", c.valueType, c.env, ok, err)
		}
	}
}

func TestEnvVar_ValidDurationStillReads(t *testing.T) {
	got, ok, err := envVarClient(t, ValueTypeDuration, "PT1.5S").GetDurationValue("env.cfg", nil)
	if err != nil || !ok || got.Milliseconds() != 1500 {
		t.Fatalf("GetDurationValue(PT1.5S) = %v ok=%v err=%v, want 1.5s", got, ok, err)
	}
}
