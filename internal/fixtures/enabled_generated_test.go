// Code generated from integration-test-data/tests/eval/enabled.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns the correct value for a simple flag
func TestEnabled_ReturnsTheCorrectValueForASimpleFlag(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.simple", nil)
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.simple")
}

// always returns false for a non-boolean flag
func TestEnabled_AlwaysReturnsFalseForANonBooleanFlag(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.integer", nil)
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.integer")
}

// returns false for a flag key that does not exist
func TestEnabled_ReturnsFalseForAFlagKeyThatDoesNotExist(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("my-missing-key", nil)
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "my-missing-key")
}

// returns false for a flag key that does not exist with a context
func TestEnabled_ReturnsFalseForAFlagKeyThatDoesNotExistWithAContext(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("my-missing-key", contextSet(map[string]map[string]interface{}{"user": {"key": "michael", "email": "michael@example.com"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "my-missing-key")
}

// returns true for a PROP_IS_ONE_OF rule when any prop matches
func TestEnabled_ReturnsTrueForAPROPISONEOFRuleWhenAnyPropMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.properties.positive", contextSet(map[string]map[string]interface{}{"": {"name": "michael", "domain": "something.com"}}))
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.properties.positive")
}

// returns false for a PROP_IS_ONE_OF rule when no prop matches
func TestEnabled_ReturnsFalseForAPROPISONEOFRuleWhenNoPropMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.properties.positive", contextSet(map[string]map[string]interface{}{"": {"name": "lauren", "domain": "something.com"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.properties.positive")
}

// returns true for a PROP_IS_NOT_ONE_OF rule when any prop doesn't match
func TestEnabled_ReturnsTrueForAPROPISNOTONEOFRuleWhenAnyPropDoesnTMatch(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.properties.negative", contextSet(map[string]map[string]interface{}{"": {"name": "lauren", "domain": "prefab.cloud"}}))
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.properties.negative")
}

// returns false for a PROP_IS_NOT_ONE_OF rule when all props match
func TestEnabled_ReturnsFalseForAPROPISNOTONEOFRuleWhenAllPropsMatch(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.properties.negative", contextSet(map[string]map[string]interface{}{"": {"name": "michael", "domain": "prefab.cloud"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.properties.negative")
}

// returns true for PROP_ENDS_WITH_ONE_OF rule when the given prop has a matching suffix
func TestEnabled_ReturnsTrueForPROPENDSWITHONEOFRuleWhenTheGivenPropHasAMatchingSuffix(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"": {"email": "jeff@prefab.cloud"}})).FeatureIsOn("feature-flag.ends-with-one-of.positive")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.ends-with-one-of.positive")
}

// returns false for PROP_ENDS_WITH_ONE_OF rule when the given prop doesn't have a matching suffix
func TestEnabled_ReturnsFalseForPROPENDSWITHONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingSuffix(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.ends-with-one-of.positive", contextSet(map[string]map[string]interface{}{"": {"email": "jeff@test.com"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.ends-with-one-of.positive")
}

// returns true for PROP_DOES_NOT_END_WITH_ONE_OF rule when the given prop doesn't have a matching suffix
func TestEnabled_ReturnsTrueForPROPDOESNOTENDWITHONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingSuffix(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"": {"email": "michael@test.com"}})).FeatureIsOn("feature-flag.ends-with-one-of.negative")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.ends-with-one-of.negative")
}

// returns false for PROP_DOES_NOT_END_WITH_ONE_OF rule when the given prop has a matching suffix
func TestEnabled_ReturnsFalseForPROPDOESNOTENDWITHONEOFRuleWhenTheGivenPropHasAMatchingSuffix(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.ends-with-one-of.negative", contextSet(map[string]map[string]interface{}{"": {"email": "michael@prefab.cloud"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.ends-with-one-of.negative")
}

// returns true for PROP_STARTS_WITH_ONE_OF rule when the given prop has a matching prefix
func TestEnabled_ReturnsTrueForPROPSTARTSWITHONEOFRuleWhenTheGivenPropHasAMatchingPrefix(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "foo@prefab.cloud"}})).FeatureIsOn("feature-flag.starts-with-one-of.positive")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.starts-with-one-of.positive")
}

// returns false for PROP_STARTS_WITH_ONE_OF rule when the given prop doesn't have a matching prefix
func TestEnabled_ReturnsFalseForPROPSTARTSWITHONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingPrefix(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "notfoo@prefab.cloud"}})).FeatureIsOn("feature-flag.starts-with-one-of.positive")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.starts-with-one-of.positive")
}

// returns true for PROP_DOES_NOT_START_WITH_ONE_OF rule when the given prop doesn't have a matching prefix
func TestEnabled_ReturnsTrueForPROPDOESNOTSTARTWITHONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingPrefix(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "notfoo@prefab.cloud"}})).FeatureIsOn("feature-flag.starts-with-one-of.negative")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.starts-with-one-of.negative")
}

// returns false for PROP_DOES_NOT_START_WITH_ONE_OF rule when the given prop has a matching prefix
func TestEnabled_ReturnsFalseForPROPDOESNOTSTARTWITHONEOFRuleWhenTheGivenPropHasAMatchingPrefix(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "foo@prefab.cloud"}})).FeatureIsOn("feature-flag.starts-with-one-of.negative")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.starts-with-one-of.negative")
}

// returns true for PROP_CONTAINS_ONE_OF rule when the given prop has a matching substring
func TestEnabled_ReturnsTrueForPROPCONTAINSONEOFRuleWhenTheGivenPropHasAMatchingSubstring(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "somefoo@prefab.cloud"}})).FeatureIsOn("feature-flag.contains-one-of.positive")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.contains-one-of.positive")
}

// returns false for PROP_CONTAINS_ONE_OF rule when the given prop doesn't have a matching substring
func TestEnabled_ReturnsFalseForPROPCONTAINSONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingSubstring(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "info@prefab.cloud"}})).FeatureIsOn("feature-flag.contains-one-of.positive")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.contains-one-of.positive")
}

// returns true for PROP_DOES_NOT_CONTAIN_ONE_OF rule when the given prop doesn't have a matching substring
func TestEnabled_ReturnsTrueForPROPDOESNOTCONTAINONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingSubstring(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "info@prefab.cloud"}})).FeatureIsOn("feature-flag.contains-one-of.negative")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.contains-one-of.negative")
}

// returns false for PROP_DOES_NOT_CONTAIN_ONE_OF rule when the given prop has a matching substring
func TestEnabled_ReturnsFalseForPROPDOESNOTCONTAINONEOFRuleWhenTheGivenPropHasAMatchingSubstring(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "notfoo@prefab.cloud"}})).FeatureIsOn("feature-flag.contains-one-of.negative")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.contains-one-of.negative")
}

// returns true for IN_SEG when the segment rule matches
func TestEnabled_ReturnsTrueForINSEGWhenTheSegmentRuleMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "lauren"}})).FeatureIsOn("feature-flag.in-segment.positive")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-segment.positive")
}

// returns false for IN_SEG when the segment rule doesn't match
func TestEnabled_ReturnsFalseForINSEGWhenTheSegmentRuleDoesnTMatch(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.in-segment.positive", contextSet(map[string]map[string]interface{}{"user": {"key": "josh"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-segment.positive")
}

// returns false for IN_SEG if any segment rule fails to match
func TestEnabled_ReturnsFalseForINSEGIfAnySegmentRuleFailsToMatch(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "prefab.cloud"}})).FeatureIsOn("feature-flag.in-seg.segment-and")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns true for IN_SEG (segment-and) if all rules matches
func TestEnabled_ReturnsTrueForINSEGSegmentAndIfAllRulesMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.in-seg.segment-and", contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "prefab.cloud"}}))
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns true for IN_SEG (segment-or) if any segment rule matches (lookup)
func TestEnabled_ReturnsTrueForINSEGSegmentOrIfAnySegmentRuleMatchesLookup(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "example.com"}})).FeatureIsOn("feature-flag.in-seg.segment-or")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-or")
}

// returns true for IN_SEG (segment-or) if any segment rule matches (prop)
func TestEnabled_ReturnsTrueForINSEGSegmentOrIfAnySegmentRuleMatchesProp(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.in-seg.segment-or", contextSet(map[string]map[string]interface{}{"user": {"key": "nobody"}, "": {"domain": "gmail.com"}}))
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-or")
}

// returns true for NOT_IN_SEG when the segment rule doesn't match
func TestEnabled_ReturnsTrueForNOTINSEGWhenTheSegmentRuleDoesnTMatch(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "josh"}})).FeatureIsOn("feature-flag.in-segment.negative")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-segment.negative")
}

// returns false for NOT_IN_SEG when the segment rule matches
func TestEnabled_ReturnsFalseForNOTINSEGWhenTheSegmentRuleMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.in-segment.negative", contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-segment.negative")
}

// returns false for NOT_IN_SEG if any segment rule matches
func TestEnabled_ReturnsFalseForNOTINSEGIfAnySegmentRuleMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "prefab.cloud"}})).FeatureIsOn("feature-flag.in-segment.multiple-criteria.negative")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-segment.multiple-criteria.negative")
}

// returns true for NOT_IN_SEG if no segment rule matches
func TestEnabled_ReturnsTrueForNOTINSEGIfNoSegmentRuleMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.in-segment.multiple-criteria.negative", contextSet(map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "something.com"}}))
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-segment.multiple-criteria.negative")
}

// returns true for NOT_IN_SEG (segment-and) if not segment rule fails to match
func TestEnabled_ReturnsTrueForNOTINSEGSegmentAndIfNotSegmentRuleFailsToMatch(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "prefab.cloud"}})).FeatureIsOn("feature-flag.not-in-seg.segment-and")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.not-in-seg.segment-and")
}

// returns true for IN_SEG (segment-and) if not segment rule fails to match
func TestEnabled_ReturnsTrueForINSEGSegmentAndIfNotSegmentRuleFailsToMatch(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.in-seg.segment-and", contextSet(map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "prefab.cloud"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns false for NOT_IN_SEG (segment-and) if segment rules matches
func TestEnabled_ReturnsFalseForNOTINSEGSegmentAndIfSegmentRulesMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "prefab.cloud"}})).FeatureIsOn("feature-flag.not-in-seg.segment-and")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.not-in-seg.segment-and")
}

// returns true for NOT_IN_SEG (segment-or) if no segment rule matches
func TestEnabled_ReturnsTrueForNOTINSEGSegmentOrIfNoSegmentRuleMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.not-in-seg.segment-or", contextSet(map[string]map[string]interface{}{"user": {"key": "nobody"}, "": {"domain": "example.com"}}))
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.not-in-seg.segment-or")
}

// returns false for NOT_IN_SEG (segment-or) if one segment rule matches (prop)
func TestEnabled_ReturnsFalseForNOTINSEGSegmentOrIfOneSegmentRuleMatchesProp(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "nobody"}, "": {"domain": "gmail.com"}})).FeatureIsOn("feature-flag.not-in-seg.segment-or")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.not-in-seg.segment-or")
}

// returns false for NOT_IN_SEG (segment-or) if one segment rule matches (lookup)
func TestEnabled_ReturnsFalseForNOTINSEGSegmentOrIfOneSegmentRuleMatchesLookup(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.not-in-seg.segment-or", contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "example.com"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.not-in-seg.segment-or")
}

// returns true for PROP_BEFORE rule when the given prop represents a date (string) before the rule's time
func TestEnabled_ReturnsTrueForPROPBEFORERuleWhenTheGivenPropRepresentsADateStringBeforeTheRuleSTime(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "2024-11-01T00:00:00Z"}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns true for PROP_BEFORE rule when the given prop represents a date (number) before the rule's time
func TestEnabled_ReturnsTrueForPROPBEFORERuleWhenTheGivenPropRepresentsADateNumberBeforeTheRuleSTime(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": 1730419200000}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns false for PROP_BEFORE rule when the given prop represents a date (number) exactly matching rule's time
func TestEnabled_ReturnsFalseForPROPBEFORERuleWhenTheGivenPropRepresentsADateNumberExactlyMatchingRuleSTime(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": 1733011200000}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns false for PROP_BEFORE rule when the given prop represents a date (number) AFTER the rule's time
func TestEnabled_ReturnsFalseForPROPBEFORERuleWhenTheGivenPropRepresentsADateNumberAFTERTheRuleSTime(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "2025-01-01T00:00:00Z"}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns false for PROP_BEFORE rule when the given prop won't parse as a date
func TestEnabled_ReturnsFalseForPROPBEFORERuleWhenTheGivenPropWonTParseAsADate(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "not a date"}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns false for PROP_BEFORE rule using current-time relative to 2050-01-01
func TestEnabled_ReturnsFalseForPROPBEFORERuleUsingCurrentTimeRelativeTo20500101(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.before.current-time", nil)
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.before.current-time")
}

// returns true for PROP_AFTER rule when the given prop represents a date (string) after the rule's time
func TestEnabled_ReturnsTrueForPROPAFTERRuleWhenTheGivenPropRepresentsADateStringAfterTheRuleSTime(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "2025-01-01T00:00:00Z"}})).FeatureIsOn("feature-flag.after")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.after")
}

// returns true for PROP_AFTER rule when the given prop represents a date (number) after the rule's time
func TestEnabled_ReturnsTrueForPROPAFTERRuleWhenTheGivenPropRepresentsADateNumberAfterTheRuleSTime(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": 1735689600000}})).FeatureIsOn("feature-flag.after")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.after")
}

// returns false for PROP_AFTER rule when the given prop represents a date (number) exactly matching rule's time
func TestEnabled_ReturnsFalseForPROPAFTERRuleWhenTheGivenPropRepresentsADateNumberExactlyMatchingRuleSTime(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": 1733011200000}})).FeatureIsOn("feature-flag.after")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.after")
}

// returns false for PROP_BEFORE rule when the given prop represents a date (number) BEFORE the rule's time
func TestEnabled_ReturnsFalseForPROPBEFORERuleWhenTheGivenPropRepresentsADateNumberBEFORETheRuleSTime(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "2024-01-01T00:00:00Z"}})).FeatureIsOn("feature-flag.after")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.after")
}

// returns false for PROP_AFTER rule when the given prop won't parse as a date
func TestEnabled_ReturnsFalseForPROPAFTERRuleWhenTheGivenPropWonTParseAsADate(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "not a date"}})).FeatureIsOn("feature-flag.after")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.after")
}

// returns false for PROP_AFTER rule using current-time relative to 2025-01-01
func TestEnabled_ReturnsFalseForPROPAFTERRuleUsingCurrentTimeRelativeTo20250101(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.after.current-time", nil)
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.after.current-time")
}

// returns true for PROP_LESS_THAN rule when the given prop is less than the rule's value
func TestEnabled_ReturnsTrueForPROPLESSTHANRuleWhenTheGivenPropIsLessThanTheRuleSValue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 20}})).FeatureIsOn("feature-flag.less-than")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.less-than")
}

// returns true for PROP_LESS_THAN rule when the given prop is less than the rule's value (float)
func TestEnabled_ReturnsTrueForPROPLESSTHANRuleWhenTheGivenPropIsLessThanTheRuleSValueFloat(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 20.5}})).FeatureIsOn("feature-flag.less-than")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.less-than")
}

// returns false for PROP_LESS_THAN rule when the given prop is equal to rule's value
func TestEnabled_ReturnsFalseForPROPLESSTHANRuleWhenTheGivenPropIsEqualToRuleSValue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 30}})).FeatureIsOn("feature-flag.less-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.less-than")
}

// returns false for PROP_LESS_THAN rule when the given prop a string
func TestEnabled_ReturnsFalseForPROPLESSTHANRuleWhenTheGivenPropAString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": "20"}})).FeatureIsOn("feature-flag.less-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.less-than")
}

// returns true for PROP_LESS_THAN_OR_EQUAL rule when the given prop is less than the rule's value
func TestEnabled_ReturnsTrueForPROPLESSTHANOREQUALRuleWhenTheGivenPropIsLessThanTheRuleSValue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 20}})).FeatureIsOn("feature-flag.less-than-or-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.less-than-or-equal")
}

// returns true for PROP_LESS_THAN_OR_EQUAL rule when the given prop is less than the rule's value (float)
func TestEnabled_ReturnsTrueForPROPLESSTHANOREQUALRuleWhenTheGivenPropIsLessThanTheRuleSValueFloat(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 20.5}})).FeatureIsOn("feature-flag.less-than-or-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.less-than-or-equal")
}

// returns false for PROP_LESS_THAN_OR_EQUAL rule when the given prop is equal to rule's value
func TestEnabled_ReturnsFalseForPROPLESSTHANOREQUALRuleWhenTheGivenPropIsEqualToRuleSValue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 30}})).FeatureIsOn("feature-flag.less-than-or-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.less-than-or-equal")
}

// returns false for PROP_LESS_THAN_OR_EQUAL rule when the given prop a string
func TestEnabled_ReturnsFalseForPROPLESSTHANOREQUALRuleWhenTheGivenPropAString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": "20"}})).FeatureIsOn("feature-flag.less-than-or-equal")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.less-than-or-equal")
}

// returns true for PROP_GREATER_THAN rule when the given prop is greater than the rule's value
func TestEnabled_ReturnsTrueForPROPGREATERTHANRuleWhenTheGivenPropIsGreaterThanTheRuleSValue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 100}})).FeatureIsOn("feature-flag.greater-than")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.greater-than")
}

// returns true for PROP_GREATER_THAN rule when the given prop is greater than the rule's value (float)
func TestEnabled_ReturnsTrueForPROPGREATERTHANRuleWhenTheGivenPropIsGreaterThanTheRuleSValueFloat(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 30.5}})).FeatureIsOn("feature-flag.greater-than")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.greater-than")
}

// returns true for PROP_GREATER_THAN rule when the given prop is greater than the rule's float value (float)
func TestEnabled_ReturnsTrueForPROPGREATERTHANRuleWhenTheGivenPropIsGreaterThanTheRuleSFloatValueFloat(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 32.7}})).FeatureIsOn("feature-flag.greater-than.double")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.greater-than.double")
}

// returns true for PROP_GREATER_THAN rule when the given prop is greater than the rule's float value (integer)
func TestEnabled_ReturnsTrueForPROPGREATERTHANRuleWhenTheGivenPropIsGreaterThanTheRuleSFloatValueInteger(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 32}})).FeatureIsOn("feature-flag.greater-than.double")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.greater-than.double")
}

// returns false for PROP_GREATER_THAN rule when the given prop is equal to rule's value
func TestEnabled_ReturnsFalseForPROPGREATERTHANRuleWhenTheGivenPropIsEqualToRuleSValue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 30}})).FeatureIsOn("feature-flag.greater-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.greater-than")
}

// returns false for PROP_GREATER_THAN rule when the given prop a string
func TestEnabled_ReturnsFalseForPROPGREATERTHANRuleWhenTheGivenPropAString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": "100"}})).FeatureIsOn("feature-flag.greater-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.greater-than")
}

// returns true for PROP_GREATER_THAN_OR_EQUAL rule when the given prop is greater than the rule's value
func TestEnabled_ReturnsTrueForPROPGREATERTHANOREQUALRuleWhenTheGivenPropIsGreaterThanTheRuleSValue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 30}})).FeatureIsOn("feature-flag.greater-than-or-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.greater-than-or-equal")
}

// returns true for PROP_GREATER_THAN_OR_EQUAL rule when the given prop is greater than the rule's value (float)
func TestEnabled_ReturnsTrueForPROPGREATERTHANOREQUALRuleWhenTheGivenPropIsGreaterThanTheRuleSValueFloat(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 30.5}})).FeatureIsOn("feature-flag.greater-than-or-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.greater-than-or-equal")
}

// returns true for PROP_GREATER_THAN_OR_EQUAL rule when the given prop is equal to rule's value
func TestEnabled_ReturnsTrueForPROPGREATERTHANOREQUALRuleWhenTheGivenPropIsEqualToRuleSValue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": 30}})).FeatureIsOn("feature-flag.greater-than-or-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.greater-than-or-equal")
}

// returns false for PROP_GREATER_THAN_OR_EQUAL rule when the given prop a string
func TestEnabled_ReturnsFalseForPROPGREATERTHANOREQUALRuleWhenTheGivenPropAString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": "100"}})).FeatureIsOn("feature-flag.greater-than-or-equal")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.greater-than-or-equal")
}

// returns true for PROP_MATCHES rule when the given prop matches the regex
func TestEnabled_ReturnsTrueForPROPMATCHESRuleWhenTheGivenPropMatchesTheRegex(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"code": "aaaaaab"}})).FeatureIsOn("feature-flag.matches")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.matches")
}

// returns false for PROP_MATCHES rule when the given prop does not match the regex
func TestEnabled_ReturnsFalseForPROPMATCHESRuleWhenTheGivenPropDoesNotMatchTheRegex(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"code": "aa"}})).FeatureIsOn("feature-flag.matches")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.matches")
}

// returns true for PROP_DOES_NOT_MATCH rule when the given prop does not match the regex
func TestEnabled_ReturnsTrueForPROPDOESNOTMATCHRuleWhenTheGivenPropDoesNotMatchTheRegex(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"code": "b"}})).FeatureIsOn("feature-flag.does-not-match")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.does-not-match")
}

// returns false for PROP_DOES_NOT_MATCH rule when the given prop matches the regex
func TestEnabled_ReturnsFalseForPROPDOESNOTMATCHRuleWhenTheGivenPropMatchesTheRegex(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"code": "aabb"}})).FeatureIsOn("feature-flag.does-not-match")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.does-not-match")
}

// returns true for IS_PRESENT rule when the given prop is a non-empty string
func TestEnabled_ReturnsTrueForISPRESENTRuleWhenTheGivenPropIsANonEmptyString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"id": "abc"}})).FeatureIsOn("feature-flag.is-present")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-present")
}

// returns true for IS_PRESENT rule when the given prop is an empty string
func TestEnabled_ReturnsTrueForISPRESENTRuleWhenTheGivenPropIsAnEmptyString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"id": ""}})).FeatureIsOn("feature-flag.is-present")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-present")
}

// returns true for IS_PRESENT rule when the given prop is the integer zero
func TestEnabled_ReturnsTrueForISPRESENTRuleWhenTheGivenPropIsTheIntegerZero(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"id": 0}})).FeatureIsOn("feature-flag.is-present")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-present")
}

// returns true for IS_PRESENT rule when the given prop is boolean false
func TestEnabled_ReturnsTrueForISPRESENTRuleWhenTheGivenPropIsBooleanFalse(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"id": false}})).FeatureIsOn("feature-flag.is-present")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-present")
}

// returns false for IS_PRESENT rule when the given prop is null
func TestEnabled_ReturnsFalseForISPRESENTRuleWhenTheGivenPropIsNull(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"id": nil}})).FeatureIsOn("feature-flag.is-present")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.is-present")
}

// returns false for IS_PRESENT rule when the given prop key is missing from the context
func TestEnabled_ReturnsFalseForISPRESENTRuleWhenTheGivenPropKeyIsMissingFromTheContext(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"name": "bob"}})).FeatureIsOn("feature-flag.is-present")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.is-present")
}

// returns false for IS_PRESENT rule when no contexts are provided at all
func TestEnabled_ReturnsFalseForISPRESENTRuleWhenNoContextsAreProvidedAtAll(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("feature-flag.is-present", nil)
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.is-present")
}

// returns false for IS_NOT_PRESENT rule when the given prop is a non-empty string
func TestEnabled_ReturnsFalseForISNOTPRESENTRuleWhenTheGivenPropIsANonEmptyString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"id": "abc"}})).FeatureIsOn("feature-flag.is-not-present")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.is-not-present")
}

// returns true for IS_NOT_PRESENT rule when the given prop is null
func TestEnabled_ReturnsTrueForISNOTPRESENTRuleWhenTheGivenPropIsNull(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"id": nil}})).FeatureIsOn("feature-flag.is-not-present")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-not-present")
}

// returns true for IS_NOT_PRESENT rule when the given prop key is missing from the context
func TestEnabled_ReturnsTrueForISNOTPRESENTRuleWhenTheGivenPropKeyIsMissingFromTheContext(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"name": "bob"}})).FeatureIsOn("feature-flag.is-not-present")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-not-present")
}

// returns true for IS_PRESENT rule on a nested path when the nested prop is set
func TestEnabled_ReturnsTrueForISPRESENTRuleOnANestedPathWhenTheNestedPropIsSet(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"organization": {"domain": "example.com"}})).FeatureIsOn("feature-flag.is-present-nested")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-present-nested")
}

// returns false for IS_PRESENT rule on a nested path when the nested key is missing but the parent context exists
func TestEnabled_ReturnsFalseForISPRESENTRuleOnANestedPathWhenTheNestedKeyIsMissingButTheParentContextExists(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"organization": {"name": "Acme Inc"}})).FeatureIsOn("feature-flag.is-present-nested")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.is-present-nested")
}

// returns false for IS_PRESENT rule on a nested path when the parent context is entirely absent
func TestEnabled_ReturnsFalseForISPRESENTRuleOnANestedPathWhenTheParentContextIsEntirelyAbsent(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"id": "abc"}})).FeatureIsOn("feature-flag.is-present-nested")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.is-present-nested")
}

// returns true for PROP_SEMVER_EQUAL rule when the given prop equals the version
func TestEnabled_ReturnsTrueForPROPSEMVEREQUALRuleWhenTheGivenPropEqualsTheVersion(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0.0"}})).FeatureIsOn("feature-flag.semver-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.semver-equal")
}

// returns false for PROP_SEMVER_EQUAL rule when the given prop does not equal the version
func TestEnabled_ReturnsFalseForPROPSEMVEREQUALRuleWhenTheGivenPropDoesNotEqualTheVersion(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0.1"}})).FeatureIsOn("feature-flag.semver-equal")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.semver-equal")
}

// returns true for PROP_SEMVER_EQUAL rule when the given prop is an abbreviated major.minor version
func TestEnabled_ReturnsTrueForPROPSEMVEREQUALRuleWhenTheGivenPropIsAnAbbreviatedMajorMinorVersion(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0"}})).FeatureIsOn("feature-flag.semver-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.semver-equal")
}

// returns true for PROP_SEMVER_LESS_THAN rule when the given prop is less than 2.0.0
func TestEnabled_ReturnsTrueForPROPSEMVERLESSTHANRuleWhenTheGivenPropIsLessThan200(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "1.5.1"}})).FeatureIsOn("feature-flag.semver-less-than")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.semver-less-than")
}

// returns false for PROP_SEMVER_LESS_THAN rule when the given prop equals the version
func TestEnabled_ReturnsFalseForPROPSEMVERLESSTHANRuleWhenTheGivenPropEqualsTheVersion(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0.0"}})).FeatureIsOn("feature-flag.semver-less-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.semver-less-than")
}

// returns false for PROP_SEMVER_LESS_THAN rule when the given prop is greater than the version
func TestEnabled_ReturnsFalseForPROPSEMVERLESSTHANRuleWhenTheGivenPropIsGreaterThanTheVersion(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.2.1"}})).FeatureIsOn("feature-flag.semver-less-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.semver-less-than")
}

// returns true for PROP_SEMVER_GREATER_THAN rule when the given prop is greater than 2.0.0
func TestEnabled_ReturnsTrueForPROPSEMVERGREATERTHANRuleWhenTheGivenPropIsGreaterThan200(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.5.1"}})).FeatureIsOn("feature-flag.semver-greater-than")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.semver-greater-than")
}

// returns false for PROP_SEMVER_GREATER_THAN rule when the given prop equals the version
func TestEnabled_ReturnsFalseForPROPSEMVERGREATERTHANRuleWhenTheGivenPropEqualsTheVersion(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0.0"}})).FeatureIsOn("feature-flag.semver-greater-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.semver-greater-than")
}

// returns false for PROP_SEMVER_EQUAL rule when the given prop is less than the version
func TestEnabled_ReturnsFalseForPROPSEMVEREQUALRuleWhenTheGivenPropIsLessThanTheVersion(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "0.0.5"}})).FeatureIsOn("feature-flag.semver-greater-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.semver-greater-than")
}

// returns true for PROP_IS_ONE_OF ["true"] when the context value is boolean true
func TestEnabled_ReturnsTrueForPROPISONEOFTrueWhenTheContextValueIsBooleanTrue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"beta": true}})).FeatureIsOn("feature-flag.is-one-of.bool")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-one-of.bool")
}

// returns false for PROP_IS_ONE_OF ["true"] when the context value is boolean false
func TestEnabled_ReturnsFalseForPROPISONEOFTrueWhenTheContextValueIsBooleanFalse(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"beta": false}})).FeatureIsOn("feature-flag.is-one-of.bool")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.is-one-of.bool")
}

// returns true for PROP_IS_ONE_OF ["true"] when the context value is the string "true"
func TestEnabled_ReturnsTrueForPROPISONEOFTrueWhenTheContextValueIsTheStringTrue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"beta": "true"}})).FeatureIsOn("feature-flag.is-one-of.bool")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-one-of.bool")
}

// returns false for PROP_IS_NOT_ONE_OF ["true"] when the context value is boolean true
func TestEnabled_ReturnsFalseForPROPISNOTONEOFTrueWhenTheContextValueIsBooleanTrue(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"beta": true}})).FeatureIsOn("feature-flag.is-not-one-of.bool")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.is-not-one-of.bool")
}

// returns true for PROP_IS_ONE_OF number strings when the context value is the integer 1
func TestEnabled_ReturnsTrueForPROPISONEOFNumberStringsWhenTheContextValueIsTheInteger1(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 1}})).FeatureIsOn("feature-flag.is-one-of.number")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-one-of.number")
}

// returns true for PROP_IS_ONE_OF number strings when the context value is the number 1.5
func TestEnabled_ReturnsTrueForPROPISONEOFNumberStringsWhenTheContextValueIsTheNumber15(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 1.5}})).FeatureIsOn("feature-flag.is-one-of.number")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-one-of.number")
}

// returns true for PROP_IS_ONE_OF number strings when the context value is the integer 1234567
func TestEnabled_ReturnsTrueForPROPISONEOFNumberStringsWhenTheContextValueIsTheInteger1234567(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 1234567}})).FeatureIsOn("feature-flag.is-one-of.number")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.is-one-of.number")
}

// returns false for PROP_IS_ONE_OF number strings when the context value is the integer 2
func TestEnabled_ReturnsFalseForPROPISONEOFNumberStringsWhenTheContextValueIsTheInteger2(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 2}})).FeatureIsOn("feature-flag.is-one-of.number")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.is-one-of.number")
}

// returns true for PROP_ENDS_WITH_ONE_OF ["3"] when the context value is the integer 123
func TestEnabled_ReturnsTrueForPROPENDSWITHONEOF3WhenTheContextValueIsTheInteger123(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 123}})).FeatureIsOn("feature-flag.ends-with-one-of.number")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.ends-with-one-of.number")
}

// returns false for PROP_ENDS_WITH_ONE_OF ["3"] when the context value is the integer 124
func TestEnabled_ReturnsFalseForPROPENDSWITHONEOF3WhenTheContextValueIsTheInteger124(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 124}})).FeatureIsOn("feature-flag.ends-with-one-of.number")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.ends-with-one-of.number")
}

// returns true for IN_INT_RANGE [0, 10) when the context value is the start bound 0
func TestEnabled_ReturnsTrueForININTRANGE010WhenTheContextValueIsTheStartBound0(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 0}})).FeatureIsOn("feature-flag.in-int-range")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-int-range")
}

// returns true for IN_INT_RANGE [0, 10) when the context value is 9
func TestEnabled_ReturnsTrueForININTRANGE010WhenTheContextValueIs9(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 9}})).FeatureIsOn("feature-flag.in-int-range")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-int-range")
}

// returns false for IN_INT_RANGE [0, 10) when the context value is the end bound 10
func TestEnabled_ReturnsFalseForININTRANGE010WhenTheContextValueIsTheEndBound10(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 10}})).FeatureIsOn("feature-flag.in-int-range")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-int-range")
}

// returns false for IN_INT_RANGE [0, 10) when the context value is -1
func TestEnabled_ReturnsFalseForININTRANGE010WhenTheContextValueIs1(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": -1}})).FeatureIsOn("feature-flag.in-int-range")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-int-range")
}

// returns true for IN_INT_RANGE with no end when the context value is 5000000000
func TestEnabled_ReturnsTrueForININTRANGEWithNoEndWhenTheContextValueIs5000000000(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": 5000000000}})).FeatureIsOn("feature-flag.in-int-range.open-end")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-int-range.open-end")
}

// returns true for IN_INT_RANGE with no start when the context value is -5
func TestEnabled_ReturnsTrueForININTRANGEWithNoStartWhenTheContextValueIs5(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"n": -5}})).FeatureIsOn("feature-flag.in-int-range.open-start")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-int-range.open-start")
}

// returns false for PROP_SEMVER_EQUAL 2.0.0 when the given prop is the prerelease 2.0.0-beta.1
func TestEnabled_ReturnsFalseForPROPSEMVEREQUAL200WhenTheGivenPropIsThePrerelease200Beta1(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0.0-beta.1"}})).FeatureIsOn("feature-flag.semver-equal")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.semver-equal")
}

// returns true for PROP_SEMVER_LESS_THAN 2.0.0 when the given prop is the prerelease 2.0.0-beta.1
func TestEnabled_ReturnsTrueForPROPSEMVERLESSTHAN200WhenTheGivenPropIsThePrerelease200Beta1(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0.0-beta.1"}})).FeatureIsOn("feature-flag.semver-less-than")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.semver-less-than")
}

// returns false for PROP_SEMVER_GREATER_THAN 2.0.0 when the given prop is the prerelease 2.0.0-beta.1
func TestEnabled_ReturnsFalseForPROPSEMVERGREATERTHAN200WhenTheGivenPropIsThePrerelease200Beta1(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0.0-beta.1"}})).FeatureIsOn("feature-flag.semver-greater-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.semver-greater-than")
}

// returns true for PROP_SEMVER_EQUAL 2.0.0 when the given prop differs only by build metadata
func TestEnabled_ReturnsTrueForPROPSEMVEREQUAL200WhenTheGivenPropDiffersOnlyByBuildMetadata(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0.0+build.5"}})).FeatureIsOn("feature-flag.semver-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.semver-equal")
}

// returns true for PROP_SEMVER_LESS_THAN 2.0.0-beta when the given prop is the prerelease 2.0.0-alpha
func TestEnabled_ReturnsTrueForPROPSEMVERLESSTHAN200BetaWhenTheGivenPropIsThePrerelease200Alpha(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2.0.0-alpha"}})).FeatureIsOn("feature-flag.semver-less-than.prerelease")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.semver-less-than.prerelease")
}

// returns true for PROP_SEMVER_EQUAL 2.0.0 when the given prop is the abbreviated major version 2
func TestEnabled_ReturnsTrueForPROPSEMVEREQUAL200WhenTheGivenPropIsTheAbbreviatedMajorVersion2(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "2"}})).FeatureIsOn("feature-flag.semver-equal")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.semver-equal")
}

// returns false for PROP_SEMVER_EQUAL 2.0.0 when the given prop has a v prefix
func TestEnabled_ReturnsFalseForPROPSEMVEREQUAL200WhenTheGivenPropHasAVPrefix(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"app": {"version": "v2.0.0"}})).FeatureIsOn("feature-flag.semver-equal")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.semver-equal")
}

// returns false for PROP_BEFORE when the given prop is a date-only string
func TestEnabled_ReturnsFalseForPROPBEFOREWhenTheGivenPropIsADateOnlyString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "2024-11-01"}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns false for PROP_BEFORE when the given prop is an RFC3339 string without an offset
func TestEnabled_ReturnsFalseForPROPBEFOREWhenTheGivenPropIsAnRFC3339StringWithoutAnOffset(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "2024-11-01T00:00:00"}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns false for PROP_BEFORE when the given prop is a free-form date string
func TestEnabled_ReturnsFalseForPROPBEFOREWhenTheGivenPropIsAFreeFormDateString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "Nov 1 2024"}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns true for PROP_BEFORE when the given prop is an RFC3339 string with a numeric offset
func TestEnabled_ReturnsTrueForPROPBEFOREWhenTheGivenPropIsAnRFC3339StringWithANumericOffset(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "2024-11-01T02:00:00+02:00"}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns true for PROP_BEFORE when the given prop is an all-digit epoch-ms string
func TestEnabled_ReturnsTrueForPROPBEFOREWhenTheGivenPropIsAnAllDigitEpochMsString(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "1730419200000"}})).FeatureIsOn("feature-flag.before")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.before")
}

// returns true for PROP_BEFORE an RFC3339 criterion when the given prop is an earlier epoch-ms number
func TestEnabled_ReturnsTrueForPROPBEFOREAnRFC3339CriterionWhenTheGivenPropIsAnEarlierEpochMsNumber(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": 1730419200000}})).FeatureIsOn("feature-flag.before.rfc3339-criterion")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.before.rfc3339-criterion")
}

// returns false for PROP_BEFORE an RFC3339 criterion when the given prop is a later RFC3339 string
func TestEnabled_ReturnsFalseForPROPBEFOREAnRFC3339CriterionWhenTheGivenPropIsALaterRFC3339String(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": "2025-01-01T00:00:00Z"}})).FeatureIsOn("feature-flag.before.rfc3339-criterion")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.before.rfc3339-criterion")
}

// returns false for PROP_AFTER an all-digit criterion "999" when the given prop is 500
func TestEnabled_ReturnsFalseForPROPAFTERAnAllDigitCriterion999WhenTheGivenPropIs500(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": 500}})).FeatureIsOn("feature-flag.after.numeric-string-criterion")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.after.numeric-string-criterion")
}

// returns true for PROP_AFTER an all-digit criterion "999" when the given prop is 1000
func TestEnabled_ReturnsTrueForPROPAFTERAnAllDigitCriterion999WhenTheGivenPropIs1000(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"creation_date": 1000}})).FeatureIsOn("feature-flag.after.numeric-string-criterion")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.after.numeric-string-criterion")
}

// returns false for PROP_GREATER_THAN 30 when the given prop is a numeric string above the threshold
func TestEnabled_ReturnsFalseForPROPGREATERTHAN30WhenTheGivenPropIsANumericStringAboveTheThreshold(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"age": "35"}})).FeatureIsOn("feature-flag.greater-than")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.greater-than")
}
