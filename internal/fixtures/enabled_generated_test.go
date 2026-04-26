// Code generated from integration-test-data/tests/eval/enabled.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// returns the correct value for a simple flag
func TestEnabled_ReturnsTheCorrectValueForASimpleFlag(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.simple")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// always returns false for a non-boolean flag
func TestEnabled_AlwaysReturnsFalseForANonBooleanFlag(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.integer")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for a PROP_IS_ONE_OF rule when any prop matches
func TestEnabled_ReturnsTrueForAPROPISONEOFRuleWhenAnyPropMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.properties.positive")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"": {"name": "michael", "domain": "something.com"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for a PROP_IS_ONE_OF rule when no prop matches
func TestEnabled_ReturnsFalseForAPROPISONEOFRuleWhenNoPropMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.properties.positive")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"": {"name": "lauren", "domain": "something.com"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for a PROP_IS_NOT_ONE_OF rule when any prop doesn't match
func TestEnabled_ReturnsTrueForAPROPISNOTONEOFRuleWhenAnyPropDoesnTMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.properties.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"": {"name": "lauren", "domain": "prefab.cloud"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for a PROP_IS_NOT_ONE_OF rule when all props match
func TestEnabled_ReturnsFalseForAPROPISNOTONEOFRuleWhenAllPropsMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.properties.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"": {"name": "michael", "domain": "prefab.cloud"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_ENDS_WITH_ONE_OF rule when the given prop has a matching suffix
func TestEnabled_ReturnsTrueForPROPENDSWITHONEOFRuleWhenTheGivenPropHasAMatchingSuffix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.ends-with-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"": {"email": "jeff@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_ENDS_WITH_ONE_OF rule when the given prop doesn't have a matching suffix
func TestEnabled_ReturnsFalseForPROPENDSWITHONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingSuffix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.ends-with-one-of.positive")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"": {"email": "jeff@test.com"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_DOES_NOT_END_WITH_ONE_OF rule when the given prop doesn't have a matching suffix
func TestEnabled_ReturnsTrueForPROPDOESNOTENDWITHONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingSuffix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.ends-with-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"": {"email": "michael@test.com"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_DOES_NOT_END_WITH_ONE_OF rule when the given prop has a matching suffix
func TestEnabled_ReturnsFalseForPROPDOESNOTENDWITHONEOFRuleWhenTheGivenPropHasAMatchingSuffix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.ends-with-one-of.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"": {"email": "michael@prefab.cloud"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_STARTS_WITH_ONE_OF rule when the given prop has a matching prefix
func TestEnabled_ReturnsTrueForPROPSTARTSWITHONEOFRuleWhenTheGivenPropHasAMatchingPrefix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.starts-with-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "foo@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_STARTS_WITH_ONE_OF rule when the given prop doesn't have a matching prefix
func TestEnabled_ReturnsFalseForPROPSTARTSWITHONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingPrefix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.starts-with-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "notfoo@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_DOES_NOT_START_WITH_ONE_OF rule when the given prop doesn't have a matching prefix
func TestEnabled_ReturnsTrueForPROPDOESNOTSTARTWITHONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingPrefix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.starts-with-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "notfoo@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_DOES_NOT_START_WITH_ONE_OF rule when the given prop has a matching prefix
func TestEnabled_ReturnsFalseForPROPDOESNOTSTARTWITHONEOFRuleWhenTheGivenPropHasAMatchingPrefix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.starts-with-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "foo@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_CONTAINS_ONE_OF rule when the given prop has a matching substring
func TestEnabled_ReturnsTrueForPROPCONTAINSONEOFRuleWhenTheGivenPropHasAMatchingSubstring(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.contains-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "somefoo@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_CONTAINS_ONE_OF rule when the given prop doesn't have a matching substring
func TestEnabled_ReturnsFalseForPROPCONTAINSONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingSubstring(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.contains-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "info@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_DOES_NOT_CONTAIN_ONE_OF rule when the given prop doesn't have a matching substring
func TestEnabled_ReturnsTrueForPROPDOESNOTCONTAINONEOFRuleWhenTheGivenPropDoesnTHaveAMatchingSubstring(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.contains-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "info@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_DOES_NOT_CONTAIN_ONE_OF rule when the given prop has a matching substring
func TestEnabled_ReturnsFalseForPROPDOESNOTCONTAINONEOFRuleWhenTheGivenPropHasAMatchingSubstring(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.contains-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "notfoo@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for IN_SEG when the segment rule matches
func TestEnabled_ReturnsTrueForINSEGWhenTheSegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"key": "lauren"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for IN_SEG when the segment rule doesn't match
func TestEnabled_ReturnsFalseForINSEGWhenTheSegmentRuleDoesnTMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.positive")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "josh"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for IN_SEG if any segment rule fails to match
func TestEnabled_ReturnsFalseForINSEGIfAnySegmentRuleFailsToMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for IN_SEG (segment-and) if all rules matches
func TestEnabled_ReturnsTrueForINSEGSegmentAndIfAllRulesMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "prefab.cloud"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for IN_SEG (segment-or) if any segment rule matches (lookup)
func TestEnabled_ReturnsTrueForINSEGSegmentOrIfAnySegmentRuleMatchesLookup(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-or")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "example.com"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for IN_SEG (segment-or) if any segment rule matches (prop)
func TestEnabled_ReturnsTrueForINSEGSegmentOrIfAnySegmentRuleMatchesProp(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-or")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "nobody"}, "": {"domain": "gmail.com"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for NOT_IN_SEG when the segment rule doesn't match
func TestEnabled_ReturnsTrueForNOTINSEGWhenTheSegmentRuleDoesnTMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"key": "josh"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for NOT_IN_SEG when the segment rule matches
func TestEnabled_ReturnsFalseForNOTINSEGWhenTheSegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "michael"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for NOT_IN_SEG if any segment rule matches
func TestEnabled_ReturnsFalseForNOTINSEGIfAnySegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.multiple-criteria.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for NOT_IN_SEG if no segment rule matches
func TestEnabled_ReturnsTrueForNOTINSEGIfNoSegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.multiple-criteria.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "something.com"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for NOT_IN_SEG (segment-and) if not segment rule fails to match
func TestEnabled_ReturnsTrueForNOTINSEGSegmentAndIfNotSegmentRuleFailsToMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for IN_SEG (segment-and) if not segment rule fails to match
func TestEnabled_ReturnsTrueForINSEGSegmentAndIfNotSegmentRuleFailsToMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "josh"}, "": {"domain": "prefab.cloud"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for NOT_IN_SEG (segment-and) if segment rules matches
func TestEnabled_ReturnsFalseForNOTINSEGSegmentAndIfSegmentRulesMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for NOT_IN_SEG (segment-or) if no segment rule matches
func TestEnabled_ReturnsTrueForNOTINSEGSegmentOrIfNoSegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-or")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "nobody"}, "": {"domain": "example.com"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for NOT_IN_SEG (segment-or) if one segment rule matches (prop)
func TestEnabled_ReturnsFalseForNOTINSEGSegmentOrIfOneSegmentRuleMatchesProp(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-or")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"key": "nobody"}, "": {"domain": "gmail.com"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for NOT_IN_SEG (segment-or) if one segment rule matches (lookup)
func TestEnabled_ReturnsFalseForNOTINSEGSegmentOrIfOneSegmentRuleMatchesLookup(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-or")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "example.com"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_BEFORE rule when the given prop represents a date (string) before the rule's time
func TestEnabled_ReturnsTrueForPROPBEFORERuleWhenTheGivenPropRepresentsADateStringBeforeTheRuleSTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": "2024-11-01T00:00:00Z"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_BEFORE rule when the given prop represents a date (number) before the rule's time
func TestEnabled_ReturnsTrueForPROPBEFORERuleWhenTheGivenPropRepresentsADateNumberBeforeTheRuleSTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": 1730419200000}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_BEFORE rule when the given prop represents a date (number) exactly matching rule's time
func TestEnabled_ReturnsFalseForPROPBEFORERuleWhenTheGivenPropRepresentsADateNumberExactlyMatchingRuleSTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": 1733011200000}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_BEFORE rule when the given prop represents a date (number) AFTER the rule's time
func TestEnabled_ReturnsFalseForPROPBEFORERuleWhenTheGivenPropRepresentsADateNumberAFTERTheRuleSTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": "2025-01-01T00:00:00Z"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_BEFORE rule when the given prop won't parse as a date
func TestEnabled_ReturnsFalseForPROPBEFORERuleWhenTheGivenPropWonTParseAsADate(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": "not a date"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_BEFORE rule using current-time relative to 2050-01-01
func TestEnabled_ReturnsFalseForPROPBEFORERuleUsingCurrentTimeRelativeTo20500101(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before.current-time")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_AFTER rule when the given prop represents a date (string) after the rule's time
func TestEnabled_ReturnsTrueForPROPAFTERRuleWhenTheGivenPropRepresentsADateStringAfterTheRuleSTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": "2025-01-01T00:00:00Z"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_AFTER rule when the given prop represents a date (number) after the rule's time
func TestEnabled_ReturnsTrueForPROPAFTERRuleWhenTheGivenPropRepresentsADateNumberAfterTheRuleSTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": 1735689600000}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_AFTER rule when the given prop represents a date (number) exactly matching rule's time
func TestEnabled_ReturnsFalseForPROPAFTERRuleWhenTheGivenPropRepresentsADateNumberExactlyMatchingRuleSTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": 1733011200000}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_BEFORE rule when the given prop represents a date (number) BEFORE the rule's time
func TestEnabled_ReturnsFalseForPROPBEFORERuleWhenTheGivenPropRepresentsADateNumberBEFORETheRuleSTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": "2024-01-01T00:00:00Z"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_AFTER rule when the given prop won't parse as a date
func TestEnabled_ReturnsFalseForPROPAFTERRuleWhenTheGivenPropWonTParseAsADate(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"creation_date": "not a date"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_AFTER rule using current-time relative to 2025-01-01
func TestEnabled_ReturnsFalseForPROPAFTERRuleUsingCurrentTimeRelativeTo20250101(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after.current-time")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_LESS_THAN rule when the given prop is less than the rule's value
func TestEnabled_ReturnsTrueForPROPLESSTHANRuleWhenTheGivenPropIsLessThanTheRuleSValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 20}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_LESS_THAN rule when the given prop is less than the rule's value (float)
func TestEnabled_ReturnsTrueForPROPLESSTHANRuleWhenTheGivenPropIsLessThanTheRuleSValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 20.5}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_LESS_THAN rule when the given prop is equal to rule's value
func TestEnabled_ReturnsFalseForPROPLESSTHANRuleWhenTheGivenPropIsEqualToRuleSValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 30}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_LESS_THAN rule when the given prop a string
func TestEnabled_ReturnsFalseForPROPLESSTHANRuleWhenTheGivenPropAString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": "20"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_LESS_THAN_OR_EQUAL rule when the given prop is less than the rule's value
func TestEnabled_ReturnsTrueForPROPLESSTHANOREQUALRuleWhenTheGivenPropIsLessThanTheRuleSValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 20}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_LESS_THAN_OR_EQUAL rule when the given prop is less than the rule's value (float)
func TestEnabled_ReturnsTrueForPROPLESSTHANOREQUALRuleWhenTheGivenPropIsLessThanTheRuleSValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 20.5}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_LESS_THAN_OR_EQUAL rule when the given prop is equal to rule's value
func TestEnabled_ReturnsFalseForPROPLESSTHANOREQUALRuleWhenTheGivenPropIsEqualToRuleSValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 30}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_LESS_THAN_OR_EQUAL rule when the given prop a string
func TestEnabled_ReturnsFalseForPROPLESSTHANOREQUALRuleWhenTheGivenPropAString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": "20"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_GREATER_THAN rule when the given prop is greater than the rule's value
func TestEnabled_ReturnsTrueForPROPGREATERTHANRuleWhenTheGivenPropIsGreaterThanTheRuleSValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 100}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_GREATER_THAN rule when the given prop is greater than the rule's value (float)
func TestEnabled_ReturnsTrueForPROPGREATERTHANRuleWhenTheGivenPropIsGreaterThanTheRuleSValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 30.5}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_GREATER_THAN rule when the given prop is greater than the rule's float value (float)
func TestEnabled_ReturnsTrueForPROPGREATERTHANRuleWhenTheGivenPropIsGreaterThanTheRuleSFloatValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than.double")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 32.7}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_GREATER_THAN rule when the given prop is greater than the rule's float value (integer)
func TestEnabled_ReturnsTrueForPROPGREATERTHANRuleWhenTheGivenPropIsGreaterThanTheRuleSFloatValueInteger(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than.double")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 32}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_GREATER_THAN rule when the given prop is equal to rule's value
func TestEnabled_ReturnsFalseForPROPGREATERTHANRuleWhenTheGivenPropIsEqualToRuleSValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 30}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_GREATER_THAN rule when the given prop a string
func TestEnabled_ReturnsFalseForPROPGREATERTHANRuleWhenTheGivenPropAString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": "100"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_GREATER_THAN_OR_EQUAL rule when the given prop is greater than the rule's value
func TestEnabled_ReturnsTrueForPROPGREATERTHANOREQUALRuleWhenTheGivenPropIsGreaterThanTheRuleSValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 30}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_GREATER_THAN_OR_EQUAL rule when the given prop is greater than the rule's value (float)
func TestEnabled_ReturnsTrueForPROPGREATERTHANOREQUALRuleWhenTheGivenPropIsGreaterThanTheRuleSValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 30.5}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns true for PROP_GREATER_THAN_OR_EQUAL rule when the given prop is equal to rule's value
func TestEnabled_ReturnsTrueForPROPGREATERTHANOREQUALRuleWhenTheGivenPropIsEqualToRuleSValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": 30}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_GREATER_THAN_OR_EQUAL rule when the given prop a string
func TestEnabled_ReturnsFalseForPROPGREATERTHANOREQUALRuleWhenTheGivenPropAString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"age": "100"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_MATCHES rule when the given prop matches the regex
func TestEnabled_ReturnsTrueForPROPMATCHESRuleWhenTheGivenPropMatchesTheRegex(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.matches")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"code": "aaaaaab"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_MATCHES rule when the given prop does not match the regex
func TestEnabled_ReturnsFalseForPROPMATCHESRuleWhenTheGivenPropDoesNotMatchTheRegex(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.matches")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"code": "aa"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_DOES_NOT_MATCH rule when the given prop does not match the regex
func TestEnabled_ReturnsTrueForPROPDOESNOTMATCHRuleWhenTheGivenPropDoesNotMatchTheRegex(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.does-not-match")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"code": "b"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_DOES_NOT_MATCH rule when the given prop matches the regex
func TestEnabled_ReturnsFalseForPROPDOESNOTMATCHRuleWhenTheGivenPropMatchesTheRegex(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.does-not-match")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"code": "aabb"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_SEMVER_EQUAL rule when the given prop equals the version
func TestEnabled_ReturnsTrueForPROPSEMVEREQUALRuleWhenTheGivenPropEqualsTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"app": {"version": "2.0.0"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_SEMVER_EQUAL rule when the given prop does not equal the version
func TestEnabled_ReturnsFalseForPROPSEMVEREQUALRuleWhenTheGivenPropDoesNotEqualTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"app": {"version": "2.0.1"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_SEMVER_EQUAL rule when the given prop is not a valid semver
func TestEnabled_ReturnsFalseForPROPSEMVEREQUALRuleWhenTheGivenPropIsNotAValidSemver(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"app": {"version": "2.0"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_SEMVER_LESS_THAN rule when the given prop is less than 2.0.0
func TestEnabled_ReturnsTrueForPROPSEMVERLESSTHANRuleWhenTheGivenPropIsLessThan200(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"app": {"version": "1.5.1"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_SEMVER_LESS_THAN rule when the given prop equals the version
func TestEnabled_ReturnsFalseForPROPSEMVERLESSTHANRuleWhenTheGivenPropEqualsTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"app": {"version": "2.0.0"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_SEMVER_LESS_THAN rule when the given prop is greater than the version
func TestEnabled_ReturnsFalseForPROPSEMVERLESSTHANRuleWhenTheGivenPropIsGreaterThanTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"app": {"version": "2.2.1"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for PROP_SEMVER_GREATER_THAN rule when the given prop is greater than 2.0.0
func TestEnabled_ReturnsTrueForPROPSEMVERGREATERTHANRuleWhenTheGivenPropIsGreaterThan200(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"app": {"version": "2.5.1"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for PROP_SEMVER_GREATER_THAN rule when the given prop equals the version
func TestEnabled_ReturnsFalseForPROPSEMVERGREATERTHANRuleWhenTheGivenPropEqualsTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"app": {"version": "2.0.0"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for PROP_SEMVER_EQUAL rule when the given prop is less than the version
func TestEnabled_ReturnsFalseForPROPSEMVEREQUALRuleWhenTheGivenPropIsLessThanTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"app": {"version": "0.0.5"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}
