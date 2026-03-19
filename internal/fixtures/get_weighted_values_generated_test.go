// Code generated from integration-test-data/tests/eval/get_weighted_values.yaml. DO NOT EDIT.

package fixtures

import (
	"testing"
)

func TestGetWeightedValues_WeightedValueIsConsistent1(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"tracking_id": "a72c15f5"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 1)
}

func TestGetWeightedValues_WeightedValueIsConsistent2(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"tracking_id": "92a202f2"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 2)
}

func TestGetWeightedValues_WeightedValueIsConsistent3(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.weighted")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"tracking_id": "8f414100"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 3)
}
