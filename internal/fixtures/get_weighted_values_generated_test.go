// Code generated from integration-test-data/tests/eval/get_weighted_values.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// weighted value is consistent 1
func TestGetWeightedValues_WeightedValueIsConsistent1(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "a72c15f5"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 1)
}

// weighted value is consistent 2
func TestGetWeightedValues_WeightedValueIsConsistent2(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "92a202f2"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 2)
}

// weighted value is consistent 3
func TestGetWeightedValues_WeightedValueIsConsistent3(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "8f414100"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 3)
}

// even split ones serves first variant at low hash fraction
func TestGetWeightedValues_EvenSplitOnesServesFirstVariantAtLowHashFraction(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.even-split-ones")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "b7ff78c8"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "a")
}

// even split ones serves first variant at low hash fraction 2
func TestGetWeightedValues_EvenSplitOnesServesFirstVariantAtLowHashFraction2(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.even-split-ones")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "289f4748"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "a")
}

// even split ones serves second variant at high hash fraction
func TestGetWeightedValues_EvenSplitOnesServesSecondVariantAtHighHashFraction(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.even-split-ones")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "d60b2cb6"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "b")
}

// even split ones serves second variant at high hash fraction 2
func TestGetWeightedValues_EvenSplitOnesServesSecondVariantAtHighHashFraction2(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.even-split-ones")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "21bcfd13"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "b")
}

// non-standard sum still serves normalized true bucket
func TestGetWeightedValues_NonStandardSumStillServesNormalizedTrueBucket(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.non-standard")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "ff8adf17"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertBoolValue(t, match, true)
}

// non-standard sum still serves normalized true bucket 2
func TestGetWeightedValues_NonStandardSumStillServesNormalizedTrueBucket2(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.non-standard")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "36ef1a7a"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertBoolValue(t, match, true)
}

// non-standard sum still serves normalized false bucket
func TestGetWeightedValues_NonStandardSumStillServesNormalizedFalseBucket(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.non-standard")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "f667c76a"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertBoolValue(t, match, false)
}

// non-standard sum still serves normalized false bucket 2
func TestGetWeightedValues_NonStandardSumStillServesNormalizedFalseBucket2(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.non-standard")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"tracking_id": "7467ca21"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertBoolValue(t, match, false)
}

// weighted value with hash property missing from context hashes empty string
func TestGetWeightedValues_WeightedValueWithHashPropertyMissingFromContextHashesEmptyString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.missing-hash")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "no-tracking-id-user"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 2)
}

// weighted value with no context hashes empty string
func TestGetWeightedValues_WeightedValueWithNoContextHashesEmptyString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.missing-hash")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 2)
}

// weighted value with hash property empty string hashes empty string
func TestGetWeightedValues_WeightedValueWithHashPropertyEmptyStringHashesEmptyString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.missing-hash")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "empty-tracking-id-user", "tracking_id": ""}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 2)
}

// weighted value with zero-weight first variant and hash property missing never serves zero-weight variant
func TestGetWeightedValues_WeightedValueWithZeroWeightFirstVariantAndHashPropertyMissingNeverServesZeroWeightVariant(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.zero-first")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "no-tracking-id-user"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 2)
}

// weighted value with no hash property is random on every evaluation
func TestGetWeightedValues_WeightedValueWithNoHashPropertyIsRandomOnEveryEvaluation(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.no-hash")
	ctx := buildContextFromMaps(nil, nil, nil)
	seen := map[int64]bool{}
	for i := 0; i < 200; i++ {
		match, err := evaluateAndResolve(t, cfg, ctx)
		if err != nil {
			t.Fatalf("resolver error: %v", err)
		}
		if !match.IsMatch {
			t.Fatalf("evaluation %d: expected a value but got no match", i)
		}
		seen[match.Value.IntValue()] = true
	}
	want := []int64{1, 2}
	ok := len(seen) == len(want)
	for _, w := range want {
		if !seen[w] {
			ok = false
		}
	}
	if !ok {
		t.Errorf("after 200 evaluations expected values seen %v, got %v", want, seen)
	}
}

// weighted value with no hash property is random on every evaluation with context
func TestGetWeightedValues_WeightedValueWithNoHashPropertyIsRandomOnEveryEvaluationWithContext(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted.no-hash")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "same-user-every-time", "tracking_id": "same-tracking-id"}})
	seen := map[int64]bool{}
	for i := 0; i < 200; i++ {
		match, err := evaluateAndResolve(t, cfg, ctx)
		if err != nil {
			t.Fatalf("resolver error: %v", err)
		}
		if !match.IsMatch {
			t.Fatalf("evaluation %d: expected a value but got no match", i)
		}
		seen[match.Value.IntValue()] = true
	}
	want := []int64{1, 2}
	ok := len(seen) == len(want)
	for _, w := range want {
		if !seen[w] {
			ok = false
		}
	}
	if !ok {
		t.Errorf("after 200 evaluations expected values seen %v, got %v", want, seen)
	}
}
