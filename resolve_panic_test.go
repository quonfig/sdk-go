package quonfig

import (
	"testing"
)

type panickingEvaluator struct{}

func (panickingEvaluator) EvaluateConfigResponse(*ConfigResponse, string, *ContextSet) *EvalResult {
	panic("boom: simulated evaluator bug")
}

// TestResolveDetail_RecoversEvaluatorPanic guards the qfg-9dxb.1 hardening: a
// panic inside evaluation (a bug, or a malformed config the evaluator did not
// anticipate) must surface as an error on that one Get, not kill the host
// process.
func TestResolveDetail_RecoversEvaluatorPanic(t *testing.T) {
	client := detailsClient(t, []ConfigResponse{
		{
			ID:        "cfg-flag",
			Key:       "flag",
			Type:      ConfigTypeFeatureFlag,
			ValueType: ValueTypeBool,
			Default: RuleSet{Rules: []Rule{
				{Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}}, Value: Value{Type: ValueTypeBool, Value: true}},
			}},
		},
	}, nil)
	client.mu.Lock()
	client.evaluator = panickingEvaluator{}
	client.mu.Unlock()

	val, ok, err := client.GetBoolValue("flag", nil)
	if err == nil {
		t.Fatal("expected an error from a panicking evaluation, got nil")
	}
	if ok || val {
		t.Errorf("GetBoolValue = (%v, %v), want (false, false)", val, ok)
	}

	d := client.EvaluateDetails("flag", nil)
	if d.Reason != ReasonError {
		t.Errorf("Reason = %v, want ReasonError", d.Reason)
	}
	if d.ErrorCode != ErrorCodeGeneral {
		t.Errorf("ErrorCode = %q, want %q", d.ErrorCode, ErrorCodeGeneral)
	}
	if d.ErrorMessage == "" {
		t.Error("ErrorMessage should describe the panic")
	}
	if d.Value != nil {
		t.Errorf("Value = %+v, want nil", d.Value)
	}
	if d.Variant != "default" {
		t.Errorf("Variant = %q, want %q", d.Variant, "default")
	}
}
