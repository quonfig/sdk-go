package evalcore

import (
	"fmt"
	"testing"
)

// fixtureWeighted mirrors integration-test-data feature-flag.weighted: it
// hashes on user.tracking_id and its first variant is 1.
func fixtureWeighted() *WeightedValuesData {
	return &WeightedValuesData{
		HashByPropertyName: "user.tracking_id",
		WeightedValues: []WeightedValue{
			{Weight: 1000, Value: Value{Type: ValueTypeInt, Value: int64(1)}},
			{Weight: 2000, Value: Value{Type: ValueTypeInt, Value: int64(3)}},
			{Weight: 97000, Value: Value{Type: ValueTypeInt, Value: int64(2)}},
		},
	}
}

// TestWeightedValueResolver_MissingHashPropertyServesFirstVariant guards
// qfg-9dxb.8: when hashByPropertyName is set but the property is absent from
// the context, the rollout serves the first variant (fraction 0.0), every
// time, instead of a random one.
func TestWeightedValueResolver_MissingHashPropertyServesFirstVariant(t *testing.T) {
	cases := map[string]ContextValueGetter{
		"nil context":            nil,
		"empty context":          EmptyContext{},
		"empty context set":      NewContextSet(),
		"named context missing":  NewContextSet().WithNamedContext(NewNamedContext("device", map[string]interface{}{"tracking_id": "d-1"})),
		"property missing":       NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{"email": "a@b.test"})),
		"named context nil data": NewContextSet().WithNamedContext(NewNamedContext("user", nil)),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 50; i++ {
				// A fresh seed each time: the result must not depend on it.
				resolver := NewWeightedValueResolver(int64(i))
				val, idx := resolver.Resolve(fixtureWeighted(), "feature-flag.weighted", ctx)
				if val == nil || idx != 0 || val.Value != int64(1) {
					t.Fatalf("iteration %d: got idx=%d val=%v, want first variant (idx 0, value 1)", i, idx, val)
				}
			}
		})
	}
}

// TestWeightedValueResolver_PresentHashPropertyPinnedToV130 proves bucketing
// is unchanged when the hash property IS present. Every expected index below
// was computed by running the released v1.3.0 evalcore.
func TestWeightedValueResolver_PresentHashPropertyPinnedToV130(t *testing.T) {
	fixturePins := map[string]int{
		"user-71":  0,
		"user-185": 0,
		"user-17":  1,
		"user-61":  1,
		"user-0":   2,
		"user-1":   2,
	}
	for id, want := range fixturePins {
		ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{"tracking_id": id}))
		if _, idx := NewWeightedValueResolver(1).Resolve(fixtureWeighted(), "feature-flag.weighted", ctx); idx != want {
			t.Errorf("feature-flag.weighted tracking_id=%q: idx=%d, want %d (v1.3.0)", id, idx, want)
		}
	}

	even := &WeightedValuesData{HashByPropertyName: "user.tracking_id"}
	for i := 0; i < 3; i++ {
		even.WeightedValues = append(even.WeightedValues, WeightedValue{Weight: 1, Value: Value{Type: ValueTypeString, Value: fmt.Sprintf("v%d", i)}})
	}
	evenPins := []struct {
		value interface{}
		want  int
	}{
		{"alice", 1}, {"bob", 2}, {"carol", 0}, {"dave", 1},
		// Present-but-empty and present-but-nil are hashed, not treated as missing.
		{"", 2}, {nil, 2},
		{int64(42), 0}, {7, 0}, {true, 2},
	}
	for _, p := range evenPins {
		ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{"tracking_id": p.value}))
		if _, idx := NewWeightedValueResolver(1).Resolve(even, "even-flag", ctx); idx != p.want {
			t.Errorf("even-flag tracking_id=%#v: idx=%d, want %d (v1.3.0)", p.value, idx, p.want)
		}
	}
}

// TestEvaluator_ReportsMissingHashProperty checks the EvalMatch signal that
// the SDK client turns into hashPropertyMissing metadata and a WARN.
func TestEvaluator_ReportsMissingHashProperty(t *testing.T) {
	cfg := &Config{
		Key: "feature-flag.weighted",
		Default: RuleSet{Rules: []Rule{{
			Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
			Value:    Value{Type: ValueTypeWeightedValues, Value: fixtureWeighted()},
		}}},
	}
	e := NewEvaluator(nil)

	missing := e.EvaluateConfig(cfg, "", nil)
	if !missing.IsWeighted || missing.WeightedValueIndex != 0 {
		t.Fatalf("missing: IsWeighted=%v idx=%d, want weighted bucket 0", missing.IsWeighted, missing.WeightedValueIndex)
	}
	if missing.MissingHashProperty != "user.tracking_id" {
		t.Errorf("missing: MissingHashProperty=%q, want %q", missing.MissingHashProperty, "user.tracking_id")
	}

	present := e.EvaluateConfig(cfg, "", NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{"tracking_id": "user-0"})))
	if present.MissingHashProperty != "" {
		t.Errorf("present: MissingHashProperty=%q, want empty", present.MissingHashProperty)
	}
}
