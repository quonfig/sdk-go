// Code generated from integration-test-data/tests/eval/get_feature_flag.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// get returns the underlying value for a feature flag
func TestGetFeatureFlag_GetReturnsTheUnderlyingValueForAFeatureFlag(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("feature-flag.integer", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.integer")
	want := int64(3)
	assert.Equal(t, want, got)
}

// get returns the underlying value for a feature flag that matches the highest precedent rule
func TestGetFeatureFlag_GetReturnsTheUnderlyingValueForAFeatureFlagThatMatchesTheHighestPrecedentRule(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("feature-flag.integer", contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.integer")
	want := int64(5)
	assert.Equal(t, want, got)
}
