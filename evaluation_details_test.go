package quonfig

import (
	"strings"
	"testing"
)

// detailsClient wires a runtime evaluator + resolver via installEnvelope so
// the EvaluateDetails path exercises the full evaluation chain (rule matching,
// weighted value resolution, variant computation, flagMetadata population).
func detailsClient(t *testing.T, configs []ConfigResponse, envLookup EnvLookupFunc) *Client {
	t.Helper()
	opts := []Option{}
	if envLookup != nil {
		opts = append(opts, WithEnvLookup(envLookup))
	}
	client, err := NewClient(opts...)
	if err != nil {
		t.Fatal(err)
	}
	client.installEnvelope(&ConfigEnvelope{
		Meta:    Meta{Version: "v1", Environment: "Production"},
		Configs: configs,
	})
	return client
}

func TestEvaluateDetails_Static_VariantAndMetadata(t *testing.T) {
	client := detailsClient(t, []ConfigResponse{
		{
			ID:        "cfg-static",
			Key:       "static.flag",
			Type:      ConfigTypeFeatureFlag,
			ValueType: ValueTypeBool,
			Default: RuleSet{Rules: []Rule{
				{Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}}, Value: Value{Type: ValueTypeBool, Value: true}},
			}},
		},
	}, nil)

	d := client.EvaluateDetails("static.flag", nil)

	if d.Reason != ReasonStatic {
		t.Fatalf("Reason = %v, want ReasonStatic", d.Reason)
	}
	if d.Variant != "static" {
		t.Errorf("Variant = %q, want %q", d.Variant, "static")
	}
	if d.ErrorCode != ErrorCodeNone {
		t.Errorf("ErrorCode = %q, want empty", d.ErrorCode)
	}
	if d.Value == nil || d.Value.BoolValue() != true {
		t.Errorf("Value = %+v, want bool true", d.Value)
	}
	if got := d.FlagMetadata["configId"]; got != "cfg-static" {
		t.Errorf("FlagMetadata[configId] = %v, want %q", got, "cfg-static")
	}
	if got := d.FlagMetadata["configType"]; got != "FEATURE_FLAG" {
		t.Errorf("FlagMetadata[configType] = %v, want %q", got, "FEATURE_FLAG")
	}
	if got := d.FlagMetadata["environment"]; got != "Production" {
		t.Errorf("FlagMetadata[environment] = %v, want %q", got, "Production")
	}
	if _, ok := d.FlagMetadata["ruleIndex"]; ok {
		t.Errorf("FlagMetadata[ruleIndex] should be omitted for STATIC, got %v", d.FlagMetadata["ruleIndex"])
	}
	if _, ok := d.FlagMetadata["weightedValueIndex"]; ok {
		t.Errorf("FlagMetadata[weightedValueIndex] should be omitted for STATIC, got %v", d.FlagMetadata["weightedValueIndex"])
	}
}

func TestEvaluateDetails_TargetingMatch_VariantAndMetadata(t *testing.T) {
	client := detailsClient(t, []ConfigResponse{
		{
			ID:        "cfg-target",
			Key:       "targeted.flag",
			Type:      ConfigTypeConfig,
			ValueType: ValueTypeBool,
			Default: RuleSet{Rules: []Rule{
				{
					Criteria: []Criterion{{
						Operator:     "PROP_IS_ONE_OF",
						PropertyName: "user.plan",
						ValueToMatch: &Value{Type: ValueTypeStringList, Value: []string{"pro"}},
					}},
					Value: Value{Type: ValueTypeBool, Value: true},
				},
				{
					Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
					Value:    Value{Type: ValueTypeBool, Value: false},
				},
			}},
		},
	}, nil)

	ctx := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"plan": "pro"})
	d := client.EvaluateDetails("targeted.flag", ctx)

	if d.Reason != ReasonTargetingMatch {
		t.Fatalf("Reason = %v, want ReasonTargetingMatch", d.Reason)
	}
	if d.Variant != "targeting:0" {
		t.Errorf("Variant = %q, want %q", d.Variant, "targeting:0")
	}
	if got := d.FlagMetadata["ruleIndex"]; got != int64(0) {
		t.Errorf("FlagMetadata[ruleIndex] = %v (%T), want int64(0)", got, got)
	}
	if got := d.FlagMetadata["configType"]; got != "CONFIG" {
		t.Errorf("FlagMetadata[configType] = %v, want %q", got, "CONFIG")
	}
	if _, ok := d.FlagMetadata["weightedValueIndex"]; ok {
		t.Errorf("FlagMetadata[weightedValueIndex] should be omitted for TARGETING_MATCH")
	}
}

func TestEvaluateDetails_Split_VariantAndMetadata(t *testing.T) {
	client := detailsClient(t, []ConfigResponse{
		{
			ID:        "cfg-split",
			Key:       "weighted.flag",
			Type:      ConfigTypeConfig,
			ValueType: ValueTypeString,
			Default: RuleSet{Rules: []Rule{
				{
					Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
					Value: Value{
						Type: ValueTypeWeightedValues,
						Value: &WeightedValuesData{
							HashByPropertyName: "user.id",
							WeightedValues: []WeightedValue{
								{Weight: 1, Value: Value{Type: ValueTypeString, Value: "variant-a"}},
								{Weight: 1, Value: Value{Type: ValueTypeString, Value: "variant-b"}},
							},
						},
					},
				},
			}},
		},
	}, nil)

	// Find a user.id that hashes into bucket index 1 so we get a non-zero
	// WeightedValueIndex (the existing detection in runtime_eval.go only flags
	// SPLIT when index > 0; using bucket 1 makes the test deterministic).
	var ctx *ContextSet
	var d EvaluationDetails
	for _, candidate := range []string{"user-a", "user-b", "user-c", "user-d", "user-e", "user-f", "user-g", "user-h"} {
		ctx = NewContextSet().WithNamedContextValues("user", map[string]interface{}{"id": candidate})
		d = client.EvaluateDetails("weighted.flag", ctx)
		if d.Reason == ReasonSplit {
			break
		}
	}

	if d.Reason != ReasonSplit {
		t.Fatalf("could not find a user.id that lands in a non-zero split bucket: Reason = %v", d.Reason)
	}
	if _, ok := d.FlagMetadata["weightedValueIndex"].(int64); !ok {
		t.Fatalf("FlagMetadata[weightedValueIndex] missing or not int64: %T %v", d.FlagMetadata["weightedValueIndex"], d.FlagMetadata["weightedValueIndex"])
	}
	if !strings.HasPrefix(d.Variant, "split:") {
		t.Errorf("Variant = %q, want split:N", d.Variant)
	}
	if got := d.FlagMetadata["ruleIndex"]; got != int64(0) {
		t.Errorf("FlagMetadata[ruleIndex] = %v (%T), want int64(0)", got, got)
	}
}

func TestEvaluateDetails_FlagNotFound_TypedErrorCode(t *testing.T) {
	client := detailsClient(t, []ConfigResponse{}, nil)

	d := client.EvaluateDetails("missing.flag", nil)

	if d.ErrorCode != ErrorCodeFlagNotFound {
		t.Fatalf("ErrorCode = %q, want %q", d.ErrorCode, ErrorCodeFlagNotFound)
	}
	if d.ErrorMessage == "" {
		t.Errorf("ErrorMessage should not be empty for missing flag")
	}
	if d.Value != nil {
		t.Errorf("Value should be nil for missing flag, got %+v", d.Value)
	}
	if d.Variant != "default" {
		t.Errorf("Variant = %q, want %q", d.Variant, "default")
	}
}

func TestEvaluateDetails_TypeMismatch_FromEnvVarCoercion(t *testing.T) {
	envLookup := func(key string) (string, bool) {
		if key == "PORT" {
			return "not-a-number", true
		}
		return "", false
	}

	client := detailsClient(t, []ConfigResponse{
		{
			ID:        "cfg-port",
			Key:       "server.port",
			Type:      ConfigTypeConfig,
			ValueType: ValueTypeInt,
			Default: RuleSet{Rules: []Rule{
				{
					Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
					Value: Value{
						Type:  ValueTypeProvided,
						Value: &ProvidedData{Source: "ENV_VAR", Lookup: "PORT"},
					},
				},
			}},
		},
	}, envLookup)

	d := client.EvaluateDetails("server.port", nil)

	if d.ErrorCode != ErrorCodeTypeMismatch {
		t.Fatalf("ErrorCode = %q, want %q (got message: %q)", d.ErrorCode, ErrorCodeTypeMismatch, d.ErrorMessage)
	}
	if d.ErrorMessage == "" {
		t.Errorf("ErrorMessage should not be empty for coerce failure")
	}
}

func TestErrorCode_StableValues(t *testing.T) {
	// The OpenFeature provider relies on these exact string values.
	// Changing them is a breaking change for downstream consumers.
	cases := map[ErrorCode]string{
		ErrorCodeNone:             "",
		ErrorCodeFlagNotFound:     "FLAG_NOT_FOUND",
		ErrorCodeTypeMismatch:     "TYPE_MISMATCH",
		ErrorCodeProviderNotReady: "PROVIDER_NOT_READY",
		ErrorCodeGeneral:          "GENERAL",
	}
	for code, want := range cases {
		if string(code) != want {
			t.Errorf("ErrorCode value = %q, want %q", string(code), want)
		}
	}
}

func TestEvaluateKey_BackwardCompat(t *testing.T) {
	// EvaluateKey's signature and semantics must not change: load-gen and
	// older consumers pin a published sdk-go and rely on the existing
	// (Value, EvalReason, bool, error) contract.
	client := detailsClient(t, []ConfigResponse{
		{
			ID:        "cfg-bc",
			Key:       "bc.flag",
			Type:      ConfigTypeFeatureFlag,
			ValueType: ValueTypeBool,
			Default: RuleSet{Rules: []Rule{
				{Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}}, Value: Value{Type: ValueTypeBool, Value: true}},
			}},
		},
	}, nil)

	val, reason, ok, err := client.EvaluateKey("bc.flag", nil)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !ok || val == nil || val.BoolValue() != true {
		t.Errorf("(val=%+v, ok=%v) want ok=true val=true", val, ok)
	}
	if reason != ReasonStatic {
		t.Errorf("reason = %v, want ReasonStatic", reason)
	}

	// Missing flag still returns ErrNotFound for errors.Is checks.
	_, _, ok, err = client.EvaluateKey("missing.bc", nil)
	if ok {
		t.Errorf("expected ok=false for missing key")
	}
	if err == nil {
		t.Errorf("expected non-nil error for missing key")
	}
}
