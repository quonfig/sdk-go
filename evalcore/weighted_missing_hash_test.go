package evalcore

import (
	"fmt"
	"sync"
	"testing"
)

// fixtureWeighted mirrors integration-test-data feature-flag.weighted: it
// hashes on user.tracking_id with variants 1 (1%), 3 (2%) and 2 (97%).
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

func userCtx(prop string, v interface{}) *ContextSet {
	return NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{prop: v}))
}

// missingShapes are the contexts in which user.tracking_id is missing
// (qfg-9dxb.8): no context, the named context absent, the property absent,
// or the value nil.
func missingShapes() map[string]ContextValueGetter {
	return map[string]ContextValueGetter{
		"nil context":           nil,
		"empty context":         EmptyContext{},
		"empty context set":     NewContextSet(),
		"named context missing": NewContextSet().WithNamedContext(NewNamedContext("device", map[string]interface{}{"tracking_id": "d-1"})),
		"property missing":      userCtx("email", "a@b.test"),
		"value nil":             userCtx("tracking_id", nil),
	}
}

// TestWeightedValueResolver_MissingHashesLikeEmpty guards qfg-9dxb.8: a
// missing hash property hashes configKey+"" exactly like a present empty
// string, so it lands in the same bucket every time. For the fixture that
// bucket is index 2 (value 2), the v1.3.0 result for tracking_id "".
func TestWeightedValueResolver_MissingHashesLikeEmpty(t *testing.T) {
	val, idx, missing := NewWeightedValueResolver(1).resolve(fixtureWeighted(), "feature-flag.weighted", userCtx("tracking_id", ""))
	if idx != 2 || val.Value != int64(2) || missing {
		t.Fatalf(`present "": idx=%d val=%v missing=%v, want idx 2 value 2 missing=false (v1.3.0)`, idx, val, missing)
	}
	for name, ctx := range missingShapes() {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 50; i++ {
				// A fresh seed each time: the result must not depend on it.
				val, idx, missing := NewWeightedValueResolver(int64(i)).resolve(fixtureWeighted(), "feature-flag.weighted", ctx)
				if idx != 2 || val.Value != int64(2) || !missing {
					t.Fatalf("iteration %d: idx=%d val=%v missing=%v, want idx 2 value 2 missing=true", i, idx, val, missing)
				}
			}
		})
	}
}

// TestWeightedValueResolver_MissingNeverServesZeroWeightFirstVariant: a
// variant with weight 0 is never served to a caller missing the hash property.
func TestWeightedValueResolver_MissingNeverServesZeroWeightFirstVariant(t *testing.T) {
	wv := &WeightedValuesData{
		HashByPropertyName: "user.id",
		WeightedValues: []WeightedValue{
			{Weight: 0, Value: Value{Type: ValueTypeString, Value: "zero"}},
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "a"}},
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "b"}},
		},
	}
	for k := 0; k < 200; k++ {
		key := fmt.Sprintf("zero-weight.%d", k)
		for name, ctx := range missingShapes() {
			val, idx := NewWeightedValueResolver(int64(k)).Resolve(wv, key, ctx)
			if idx == 0 || val.StringValue() == "zero" {
				t.Fatalf("%s / %s: served zero-weight variant (idx=%d)", key, name, idx)
			}
		}
	}
}

// TestWeightedValueResolver_NoHashPropertyIsRandomPerEvaluation: with no
// hashByPropertyName configured, each evaluation picks a random variant
// (v1.3.0 behavior), with or without a context, and concurrently race-free.
func TestWeightedValueResolver_NoHashPropertyIsRandomPerEvaluation(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "A"}},
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "B"}},
		},
	}
	resolver := NewWeightedValueResolver(7)
	for name, ctx := range map[string]ContextValueGetter{"nil context": nil, "with context": userCtx("id", "u-1")} {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			counts := map[int]int{}
			var wg sync.WaitGroup
			for g := 0; g < 10; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < 100; i++ {
						_, idx, missing := resolver.resolve(wv, "fifty-fifty", ctx)
						if missing {
							t.Errorf("hashPropertyMissing must be false with no hash property")
						}
						mu.Lock()
						counts[idx]++
						mu.Unlock()
					}
				}()
			}
			wg.Wait()
			t.Logf("1000 evaluations: A=%d B=%d", counts[0], counts[1])
			if counts[0] == 0 || counts[1] == 0 || counts[0]+counts[1] != 1000 {
				t.Fatalf("want both variants over 1000 evaluations, got %v", counts)
			}
		})
	}
}

// TestWeightedValueResolver_PresentHashPropertyPinnedToV130 proves bucketing
// is unchanged when the hash property is present with a non-empty value.
// Every expected index below was computed by running the released v1.3.0
// evalcore.
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
		if _, idx := NewWeightedValueResolver(1).Resolve(fixtureWeighted(), "feature-flag.weighted", userCtx("tracking_id", id)); idx != want {
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
		{"", 2},
		{int64(42), 0}, {7, 0}, {true, 2},
	}
	for _, p := range evenPins {
		if _, idx := NewWeightedValueResolver(1).Resolve(even, "even-flag", userCtx("tracking_id", p.value)); idx != p.want {
			t.Errorf("even-flag tracking_id=%#v: idx=%d, want %d (v1.3.0)", p.value, idx, p.want)
		}
	}
}

// TestEvaluator_ReportsMissingHashProperty checks the EvalMatch signal that
// the SDK client turns into hashPropertyMissing metadata and a WARN: set only
// when the property is missing, not when it is present (even if empty).
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
	if !missing.IsWeighted || missing.WeightedValueIndex != 2 {
		t.Fatalf("missing: IsWeighted=%v idx=%d, want weighted index 2", missing.IsWeighted, missing.WeightedValueIndex)
	}
	if missing.MissingHashProperty != "user.tracking_id" {
		t.Errorf("missing: MissingHashProperty=%q, want %q", missing.MissingHashProperty, "user.tracking_id")
	}
	for _, v := range []string{"", "user-0"} {
		present := e.EvaluateConfig(cfg, "", userCtx("tracking_id", v))
		if present.MissingHashProperty != "" {
			t.Errorf("present %q: MissingHashProperty=%q, want empty", v, present.MissingHashProperty)
		}
	}
}
