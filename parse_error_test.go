package quonfig

import (
	"encoding/json"
	"errors"
	"testing"
)

// Malformed data is PARSE_ERROR, a wrong-type read is TYPE_MISMATCH
// (qfg-wg1n). Malformed data is an ENV_VAR value that does not parse as the
// config's own type, or a stored int/double/duration that does not parse.

// envVarConfig is a config of valueType whose value is provided by ENV_VAR VAR.
func envVarConfig(key string, valueType ValueType) ConfigResponse {
	return ConfigResponse{
		ID:        "cfg-" + key,
		Key:       key,
		Type:      ConfigTypeConfig,
		ValueType: valueType,
		Default: RuleSet{Rules: []Rule{{
			Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
			Value:    Value{Type: ValueTypeProvided, Value: &ProvidedData{Source: "ENV_VAR", Lookup: "VAR"}},
		}}},
	}
}

func envVarLookup(val string) EnvLookupFunc {
	return func(key string) (string, bool) {
		if key == "VAR" {
			return val, true
		}
		return "", false
	}
}

// storedValue decodes a stored value the way config.go does, so a malformed
// number is kept as its raw string exactly as it would be from a workspace.
func storedValue(t *testing.T, raw string) Value {
	t.Helper()
	var v Value
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return v
}

func TestErrorCodeParseError_StableValue(t *testing.T) {
	if string(ErrorCodeParseError) != "PARSE_ERROR" {
		t.Fatalf("ErrorCodeParseError = %q, want PARSE_ERROR", ErrorCodeParseError)
	}
}

func TestEvaluateDetails_MalformedEnvVarIsParseError(t *testing.T) {
	cases := []struct {
		valueType ValueType
		env       string
	}{
		{ValueTypeInt, "twelve"},
		{ValueTypeDouble, "one point five"},
		{ValueTypeBool, "maybe"},
		{ValueTypeDuration, "ninety seconds"},
		{ValueTypeJSON, "{not json"},
	}
	for _, tc := range cases {
		t.Run(string(tc.valueType), func(t *testing.T) {
			c := detailsClient(t, []ConfigResponse{envVarConfig("k", tc.valueType)}, envVarLookup(tc.env))
			d := c.EvaluateDetails("k", nil)
			if d.ErrorCode != ErrorCodeParseError {
				t.Fatalf("ErrorCode = %q, want PARSE_ERROR (message %q)", d.ErrorCode, d.ErrorMessage)
			}
			if d.Reason != ReasonError || d.Value != nil {
				t.Fatalf("Reason = %v, Value = %+v; want ReasonError and no value", d.Reason, d.Value)
			}
		})
	}
}

// The typed getters return an error rather than an ErrorCode; errorCodeFor is
// the single mapping from those errors to a code, so pin it per getter path.
func TestTypedGetters_MalformedValueMapsToParseError(t *testing.T) {
	stored := map[string]Value{
		"int":      storedValue(t, `{"type":"int","value":"twelve"}`),
		"double":   storedValue(t, `{"type":"double","value":"one point five"}`),
		"duration": storedValue(t, `{"type":"duration","value":"ninety seconds"}`),
	}
	c := typedReadClient(t, nil, stored)
	reads := map[string]func() error{
		"GetIntValue stored int":        func() error { _, _, err := c.GetIntValue("int", nil); return err },
		"GetFloatValue stored double":   func() error { _, _, err := c.GetFloatValue("double", nil); return err },
		"GetFloatValue stored int":      func() error { _, _, err := c.GetFloatValue("int", nil); return err },
		"GetIntValue stored double":     func() error { _, _, err := c.GetIntValue("double", nil); return err },
		"GetDurationValue stored value": func() error { _, _, err := c.GetDurationValue("duration", nil); return err },
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			err := read()
			if !errors.Is(err, ErrUnableToCoerce) {
				t.Fatalf("err = %v, want ErrUnableToCoerce", err)
			}
			if got := errorCodeFor(err); got != ErrorCodeParseError {
				t.Fatalf("errorCodeFor(%v) = %q, want PARSE_ERROR", err, got)
			}
		})
	}

	envCases := []struct {
		name      string
		valueType ValueType
		env       string
		read      func(c *Client) error
	}{
		{"GetIntValue", ValueTypeInt, "twelve", func(c *Client) error { _, _, err := c.GetIntValue("k", nil); return err }},
		{"GetFloatValue", ValueTypeDouble, "x", func(c *Client) error { _, _, err := c.GetFloatValue("k", nil); return err }},
		{"GetBoolValue", ValueTypeBool, "maybe", func(c *Client) error { _, _, err := c.GetBoolValue("k", nil); return err }},
		{"GetDurationValue", ValueTypeDuration, "soon", func(c *Client) error { _, _, err := c.GetDurationValue("k", nil); return err }},
		{"GetJSONValue", ValueTypeJSON, "{", func(c *Client) error { _, _, err := c.GetJSONValue("k", nil); return err }},
	}
	for _, tc := range envCases {
		t.Run(tc.name+" env var", func(t *testing.T) {
			ec := detailsClient(t, []ConfigResponse{envVarConfig("k", tc.valueType)}, envVarLookup(tc.env))
			err := tc.read(ec)
			if got := errorCodeFor(err); got != ErrorCodeParseError {
				t.Fatalf("errorCodeFor(%v) = %q, want PARSE_ERROR", err, got)
			}
		})
	}
}

func TestTypedGetters_WrongTypeStaysTypeMismatch(t *testing.T) {
	c := typedReadClient(t, nil, map[string]Value{
		"s": {Type: ValueTypeString, Value: "12"},
		"d": {Type: ValueTypeDouble, Value: 3.5},
	})
	reads := map[string]func() error{
		"GetIntValue on string":      func() error { _, _, err := c.GetIntValue("s", nil); return err },
		"GetBoolValue on string":     func() error { _, _, err := c.GetBoolValue("s", nil); return err },
		"GetDurationValue on string": func() error { _, _, err := c.GetDurationValue("s", nil); return err },
		"GetIntValue on fraction":    func() error { _, _, err := c.GetIntValue("d", nil); return err },
		"GetStringValue on double":   func() error { _, _, err := c.GetStringValue("d", nil); return err },
	}
	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			if got := errorCodeFor(read()); got != ErrorCodeTypeMismatch {
				t.Fatalf("errorCodeFor = %q, want TYPE_MISMATCH", got)
			}
		})
	}
}
