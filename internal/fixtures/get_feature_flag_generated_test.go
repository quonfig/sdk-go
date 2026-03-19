// Code generated from integration-test-data/tests/eval/get_feature_flag.yaml. DO NOT EDIT.

package fixtures

import (
	"testing"
)

func TestGetFeatureFlag_GetReturnsTheUnderlyingValueForAFeatureFlag(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.integer")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 3)
}

func TestGetFeatureFlag_GetReturnsTheUnderlyingValueForAFeatureFlagThatMatchesTheHighestPrecedentRule(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.integer")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"key": "michael"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 5)
}
