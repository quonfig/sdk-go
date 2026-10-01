// Code generated from integration-test-data/tests/eval/get_weighted_values.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// weighted value is consistent 1
func TestGetWeightedValues_WeightedValueIsConsistent1(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("feature-flag.weighted", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "a72c15f5"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted")
	want := int64(1)
	assert.Equal(t, want, got)
}

// weighted value is consistent 2
func TestGetWeightedValues_WeightedValueIsConsistent2(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("feature-flag.weighted", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "92a202f2"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted")
	want := int64(2)
	assert.Equal(t, want, got)
}

// weighted value is consistent 3
func TestGetWeightedValues_WeightedValueIsConsistent3(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("feature-flag.weighted", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "8f414100"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted")
	want := int64(3)
	assert.Equal(t, want, got)
}

// even split ones serves first variant at low hash fraction
func TestGetWeightedValues_EvenSplitOnesServesFirstVariantAtLowHashFraction(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("feature-flag.weighted.even-split-ones", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "b7ff78c8"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.even-split-ones")
	want := "a"
	assert.Equal(t, want, got)
}

// even split ones serves first variant at low hash fraction 2
func TestGetWeightedValues_EvenSplitOnesServesFirstVariantAtLowHashFraction2(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("feature-flag.weighted.even-split-ones", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "289f4748"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.even-split-ones")
	want := "a"
	assert.Equal(t, want, got)
}

// even split ones serves second variant at high hash fraction
func TestGetWeightedValues_EvenSplitOnesServesSecondVariantAtHighHashFraction(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("feature-flag.weighted.even-split-ones", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "d60b2cb6"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.even-split-ones")
	want := "b"
	assert.Equal(t, want, got)
}

// even split ones serves second variant at high hash fraction 2
func TestGetWeightedValues_EvenSplitOnesServesSecondVariantAtHighHashFraction2(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("feature-flag.weighted.even-split-ones", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "21bcfd13"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.even-split-ones")
	want := "b"
	assert.Equal(t, want, got)
}

// non-standard sum still serves normalized true bucket
func TestGetWeightedValues_NonStandardSumStillServesNormalizedTrueBucket(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetBoolValue("feature-flag.weighted.non-standard", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "ff8adf17"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.non-standard")
	want := true
	assert.Equal(t, want, got)
}

// non-standard sum still serves normalized true bucket 2
func TestGetWeightedValues_NonStandardSumStillServesNormalizedTrueBucket2(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetBoolValue("feature-flag.weighted.non-standard", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "36ef1a7a"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.non-standard")
	want := true
	assert.Equal(t, want, got)
}

// non-standard sum still serves normalized false bucket
func TestGetWeightedValues_NonStandardSumStillServesNormalizedFalseBucket(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetBoolValue("feature-flag.weighted.non-standard", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "f667c76a"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.non-standard")
	want := false
	assert.Equal(t, want, got)
}

// non-standard sum still serves normalized false bucket 2
func TestGetWeightedValues_NonStandardSumStillServesNormalizedFalseBucket2(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetBoolValue("feature-flag.weighted.non-standard", contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "7467ca21"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.non-standard")
	want := false
	assert.Equal(t, want, got)
}

// weighted value with hash property missing from context hashes empty string
func TestGetWeightedValues_WeightedValueWithHashPropertyMissingFromContextHashesEmptyString(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("feature-flag.weighted.missing-hash", contextSet(map[string]map[string]interface{}{"user": {"key": "no-tracking-id-user"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.missing-hash")
	want := int64(2)
	assert.Equal(t, want, got)
}

// weighted value with no context hashes empty string
func TestGetWeightedValues_WeightedValueWithNoContextHashesEmptyString(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("feature-flag.weighted.missing-hash", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.missing-hash")
	want := int64(2)
	assert.Equal(t, want, got)
}

// weighted value with hash property empty string hashes empty string
func TestGetWeightedValues_WeightedValueWithHashPropertyEmptyStringHashesEmptyString(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("feature-flag.weighted.missing-hash", contextSet(map[string]map[string]interface{}{"user": {"key": "empty-tracking-id-user", "tracking_id": ""}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.missing-hash")
	want := int64(2)
	assert.Equal(t, want, got)
}

// weighted value with zero-weight first variant and hash property missing never serves zero-weight variant
func TestGetWeightedValues_WeightedValueWithZeroWeightFirstVariantAndHashPropertyMissingNeverServesZeroWeightVariant(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("feature-flag.weighted.zero-first", contextSet(map[string]map[string]interface{}{"user": {"key": "no-tracking-id-user"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "feature-flag.weighted.zero-first")
	want := int64(2)
	assert.Equal(t, want, got)
}

// weighted value with no hash property is random on every evaluation
func TestGetWeightedValues_WeightedValueWithNoHashPropertyIsRandomOnEveryEvaluation(t *testing.T) {
	c := mustPublicClient(t)
	seen := map[int64]bool{}
	for i := 0; i < 200; i++ {
		got, ok, err := c.GetIntValue("feature-flag.weighted.no-hash", nil)
		require.NoError(t, err)
		require.True(t, ok, "evaluation %d of %q found no value", i, "feature-flag.weighted.no-hash")
		seen[got] = true
	}
	assert.Equal(t, map[int64]bool{int64(1): true, int64(2): true}, seen, "values seen over 200 evaluations")
}

// weighted value with no hash property is random on every evaluation with context
func TestGetWeightedValues_WeightedValueWithNoHashPropertyIsRandomOnEveryEvaluationWithContext(t *testing.T) {
	c := mustPublicClient(t)
	seen := map[int64]bool{}
	for i := 0; i < 200; i++ {
		got, ok, err := c.GetIntValue("feature-flag.weighted.no-hash", contextSet(map[string]map[string]interface{}{"user": {"key": "same-user-every-time", "tracking_id": "same-tracking-id"}}))
		require.NoError(t, err)
		require.True(t, ok, "evaluation %d of %q found no value", i, "feature-flag.weighted.no-hash")
		seen[got] = true
	}
	assert.Equal(t, map[int64]bool{int64(1): true, int64(2): true}, seen, "values seen over 200 evaluations")
}
