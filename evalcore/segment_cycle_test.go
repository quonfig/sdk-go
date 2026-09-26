package evalcore

import (
	"runtime/debug"
	"testing"
)

// segRef builds a segment config whose only matching rule is IN_SEG <ref>.
func segRef(key, ref string) *Config {
	return &Config{
		Key:       key,
		Type:      ConfigTypeSegment,
		ValueType: ValueTypeBool,
		Default: RuleSet{Rules: []Rule{
			{
				Criteria: []Criterion{{
					Operator:     OpInSeg,
					ValueToMatch: &Value{Type: ValueTypeString, Value: ref},
				}},
				Value: Value{Type: ValueTypeBool, Value: true},
			},
			{
				Criteria: []Criterion{{Operator: OpAlwaysTrue}},
				Value:    Value{Type: ValueTypeBool, Value: false},
			},
		}},
	}
}

func flagInSeg(key, seg string) *Config {
	cfg := segRef(key, seg)
	cfg.Type = ConfigTypeFeatureFlag
	return cfg
}

// A malformed workspace (hand-edited datadir, bad write) must never crash the
// process. Before the qfg-9dxb.4 fix these recursed until "fatal error: stack
// overflow", which recover() cannot catch.
func TestEvaluateConfig_SegmentCycleDoesNotCrash(t *testing.T) {
	// Keep a regression fast and small instead of growing a 1GB stack.
	defer debug.SetMaxStack(debug.SetMaxStack(64 << 20))

	cases := map[string][]*Config{
		"self-reference": {segRef("seg-a", "seg-a")},
		"two-cycle":      {segRef("seg-a", "seg-b"), segRef("seg-b", "seg-a")},
		"three-cycle":    {segRef("seg-a", "seg-b"), segRef("seg-b", "seg-c"), segRef("seg-c", "seg-a")},
	}
	for name, segs := range cases {
		t.Run(name, func(t *testing.T) {
			store := newTestConfigStore()
			for _, s := range segs {
				store.addConfig(s)
			}
			flag := flagInSeg("flag", "seg-a")
			store.addConfig(flag)
			eval := NewEvaluatorWithSeed(store, 1)

			res := eval.EvaluateConfig(flag, "", EmptyContext{})
			if !res.IsMatch || res.Value == nil {
				t.Fatalf("expected fallthrough match, got %+v", res)
			}
			if res.Value.BoolValue() {
				t.Errorf("cyclic segment must not count as a match")
			}
			// Evaluating the segment itself directly must also terminate.
			segRes := eval.EvaluateConfig(segs[0], "", EmptyContext{})
			if segRes.Value == nil || segRes.Value.BoolValue() {
				t.Errorf("cyclic segment evaluated directly must not match, got %+v", segRes)
			}
		})
	}
}

// A diamond (flag -> A, flag -> B, A -> C, B -> C) is not a cycle and must
// keep resolving: the guard tracks the current path, not every key ever seen.
func TestEvaluateConfig_SegmentDiamondStillResolves(t *testing.T) {
	store := newTestConfigStore()
	leaf := &Config{
		Key: "seg-c", Type: ConfigTypeSegment, ValueType: ValueTypeBool,
		Default: RuleSet{Rules: []Rule{{
			Criteria: []Criterion{{Operator: OpAlwaysTrue}},
			Value:    Value{Type: ValueTypeBool, Value: true},
		}}},
	}
	store.addConfig(leaf)
	store.addConfig(segRef("seg-a", "seg-c"))
	store.addConfig(segRef("seg-b", "seg-c"))
	flag := &Config{
		Key: "flag", Type: ConfigTypeFeatureFlag, ValueType: ValueTypeBool,
		Default: RuleSet{Rules: []Rule{
			{
				Criteria: []Criterion{
					{Operator: OpInSeg, ValueToMatch: &Value{Type: ValueTypeString, Value: "seg-a"}},
					{Operator: OpInSeg, ValueToMatch: &Value{Type: ValueTypeString, Value: "seg-b"}},
				},
				Value: Value{Type: ValueTypeBool, Value: true},
			},
			{
				Criteria: []Criterion{{Operator: OpAlwaysTrue}},
				Value:    Value{Type: ValueTypeBool, Value: false},
			},
		}},
	}
	store.addConfig(flag)
	res := NewEvaluatorWithSeed(store, 1).EvaluateConfig(flag, "", EmptyContext{})
	if res.Value == nil || !res.Value.BoolValue() {
		t.Fatalf("diamond segment reference should match, got %+v", res)
	}
}
