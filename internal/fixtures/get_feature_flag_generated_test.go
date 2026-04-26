// Code generated from integration-test-data/tests/eval/get_feature_flag.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// get returns the underlying value for a feature flag
func TestGetFeatureFlag_GetReturnsTheUnderlyingValueForAFeatureFlag(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.integer")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 3)
}

// get returns the underlying value for a feature flag that matches the highest precedent rule
func TestGetFeatureFlag_GetReturnsTheUnderlyingValueForAFeatureFlagThatMatchesTheHighestPrecedentRule(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.integer")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "michael"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 5)
}
