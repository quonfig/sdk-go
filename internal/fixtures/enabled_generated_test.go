// Code generated from integration-test-data/tests/eval/enabled.yaml. DO NOT EDIT.

package fixtures

import (
	"testing"
)

// ALWAYS_ON

func TestEnabled_ReturnsTheCorrectValueForASimpleFlag(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.simple")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_AlwaysReturnsFalseForANonBooleanFlag(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.integer")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_IS_ONE_OF

func TestEnabled_ReturnsTrueForPropIsOneOfRuleWhenAnyPropMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.properties.positive")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"": {
			"name":   "michael",
			"domain": "something.com",
		},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropIsOneOfRuleWhenNoPropMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.properties.positive")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"": {
			"name":   "lauren",
			"domain": "something.com",
		},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_IS_NOT_ONE_OF

func TestEnabled_ReturnsTrueForPropIsNotOneOfRuleWhenAnyPropDoesntMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.properties.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"": {
			"name":   "lauren",
			"domain": "prefab.cloud",
		},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropIsNotOneOfRuleWhenAllPropsMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.properties.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"": {
			"name":   "michael",
			"domain": "prefab.cloud",
		},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_ENDS_WITH_ONE_OF

func TestEnabled_ReturnsTrueForPropEndsWithOneOfRuleWhenMatchingSuffix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.ends-with-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"": {"email": "jeff@prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropEndsWithOneOfRuleWhenNoMatchingSuffix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.ends-with-one-of.positive")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"": {"email": "jeff@test.com"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_DOES_NOT_END_WITH_ONE_OF

func TestEnabled_ReturnsTrueForPropDoesNotEndWithOneOfRuleWhenNoMatchingSuffix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.ends-with-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"": {"email": "michael@test.com"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropDoesNotEndWithOneOfRuleWhenMatchingSuffix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.ends-with-one-of.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"": {"email": "michael@prefab.cloud"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_STARTS_WITH_ONE_OF

func TestEnabled_ReturnsTrueForPropStartsWithOneOfRuleWhenMatchingPrefix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.starts-with-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"email": "foo@prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropStartsWithOneOfRuleWhenNoMatchingPrefix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.starts-with-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"email": "notfoo@prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_DOES_NOT_START_WITH_ONE_OF

func TestEnabled_ReturnsTrueForPropDoesNotStartWithOneOfRuleWhenNoMatchingPrefix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.starts-with-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"email": "notfoo@prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropDoesNotStartWithOneOfRuleWhenMatchingPrefix(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.starts-with-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"email": "foo@prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_CONTAINS_ONE_OF

func TestEnabled_ReturnsTrueForPropContainsOneOfRuleWhenMatchingSubstring(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.contains-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"email": "somefoo@prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropContainsOneOfRuleWhenNoMatchingSubstring(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.contains-one-of.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"email": "info@prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_DOES_NOT_CONTAIN_ONE_OF

func TestEnabled_ReturnsTrueForPropDoesNotContainOneOfRuleWhenNoMatchingSubstring(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.contains-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"email": "info@prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropDoesNotContainOneOfRuleWhenMatchingSubstring(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.contains-one-of.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"email": "notfoo@prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// IN_SEG

func TestEnabled_ReturnsTrueForInSegWhenTheSegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.positive")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "lauren"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForInSegWhenTheSegmentRuleDoesntMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.positive")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"key": "josh"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForInSegIfAnySegmentRuleFailsToMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "josh"},
		"":     {"domain": "prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsTrueForInSegSegmentAndIfAllRulesMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"key": "michael"},
		"":     {"domain": "prefab.cloud"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForInSegSegmentOrIfAnySegmentRuleMatchesLookup(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-or")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "michael"},
		"":     {"domain": "example.com"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForInSegSegmentOrIfAnySegmentRuleMatchesProp(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-or")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"key": "nobody"},
		"":     {"domain": "gmail.com"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// NOT_IN_SEG

func TestEnabled_ReturnsTrueForNotInSegWhenTheSegmentRuleDoesntMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "josh"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForNotInSegWhenTheSegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"key": "michael"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForNotInSegIfAnySegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.multiple-criteria.negative")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "josh"},
		"":     {"domain": "prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForNotInSegIfNoSegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-segment.multiple-criteria.negative")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"key": "josh"},
		"":     {"domain": "something.com"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForNotInSegSegmentAndIfNotSegmentRuleFailsToMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "josh"},
		"":     {"domain": "prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForInSegSegmentAndIfNotSegmentRuleFailsToMatch(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"key": "josh"},
		"":     {"domain": "prefab.cloud"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForNotInSegSegmentAndIfSegmentRulesMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "michael"},
		"":     {"domain": "prefab.cloud"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsTrueForNotInSegSegmentOrIfNoSegmentRuleMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-or")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"key": "nobody"},
		"":     {"domain": "example.com"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForNotInSegSegmentOrIfOneSegmentRuleMatchesProp(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-or")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "nobody"},
		"":     {"domain": "gmail.com"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForNotInSegSegmentOrIfOneSegmentRuleMatchesLookup(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.not-in-seg.segment-or")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"key": "michael"},
		"":     {"domain": "example.com"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_BEFORE

func TestEnabled_ReturnsTrueForPropBeforeRuleWhenDateStringBeforeRulesTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": "2024-11-01T00:00:00Z"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropBeforeRuleWhenDateNumberBeforeRulesTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": 1730419200000},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropBeforeRuleWhenDateNumberExactlyMatchingRulesTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": 1733011200000},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropBeforeRuleWhenDateNumberAfterRulesTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": "2025-01-01T00:00:00Z"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropBeforeRuleWhenPropWontParseAsADate(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": "not a date"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropBeforeRuleUsingCurrentTimeRelativeTo2050(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.before.current-time")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// PROP_AFTER

func TestEnabled_ReturnsTrueForPropAfterRuleWhenDateStringAfterRulesTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": "2025-01-01T00:00:00Z"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropAfterRuleWhenDateNumberAfterRulesTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": 1735689600000},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropAfterRuleWhenDateNumberExactlyMatchingRulesTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": 1733011200000},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropBeforeRuleWhenDateNumberBeforeRulesTime(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": "2024-01-01T00:00:00Z"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropAfterRuleWhenPropWontParseAsADate(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"creation_date": "not a date"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropAfterRuleUsingCurrentTimeRelativeTo2025(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.after.current-time")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// PROP_LESS_THAN

func TestEnabled_ReturnsTrueForPropLessThanRuleWhenPropIsLessThanRulesValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 20},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropLessThanRuleWhenPropIsLessThanRulesValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 20.5},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropLessThanRuleWhenPropIsEqualToRulesValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 30},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropLessThanRuleWhenPropIsAString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": "20"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_LESS_THAN_OR_EQUAL

func TestEnabled_ReturnsTrueForPropLessThanOrEqualRuleWhenPropIsLessThanRulesValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 20},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropLessThanOrEqualRuleWhenPropIsLessThanRulesValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 20.5},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropLessThanOrEqualRuleWhenPropIsEqualToRulesValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 30},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropLessThanOrEqualRuleWhenPropIsAString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.less-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": "20"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_GREATER_THAN

func TestEnabled_ReturnsTrueForPropGreaterThanRuleWhenPropIsGreaterThanRulesValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 100},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropGreaterThanRuleWhenPropIsGreaterThanRulesValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 30.5},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropGreaterThanRuleWhenPropIsGreaterThanRulesFloatValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than.double")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 32.7},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropGreaterThanRuleWhenPropIsGreaterThanRulesFloatValueInteger(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than.double")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 32},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropGreaterThanRuleWhenPropIsEqualToRulesValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 30},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropGreaterThanRuleWhenPropIsAString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": "100"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_GREATER_THAN_OR_EQUAL

func TestEnabled_ReturnsTrueForPropGreaterThanOrEqualRuleWhenPropIsGreaterThanRulesValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 30},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropGreaterThanOrEqualRuleWhenPropIsGreaterThanRulesValueFloat(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 30.5},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsTrueForPropGreaterThanOrEqualRuleWhenPropIsEqualToRulesValue(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": 30},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropGreaterThanOrEqualRuleWhenPropIsAString(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.greater-than-or-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"age": "100"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_MATCHES

func TestEnabled_ReturnsTrueForPropMatchesRuleWhenPropMatchesTheRegex(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.matches")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"code": "aaaaaab"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropMatchesRuleWhenPropDoesNotMatchTheRegex(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.matches")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"code": "aa"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_DOES_NOT_MATCH

func TestEnabled_ReturnsTrueForPropDoesNotMatchRuleWhenPropDoesNotMatchTheRegex(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.does-not-match")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"code": "b"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropDoesNotMatchRuleWhenPropMatchesTheRegex(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.does-not-match")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"code": "aabb"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_SEMVER_EQUAL

func TestEnabled_ReturnsTrueForPropSemverEqualRuleWhenPropEqualsTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"app": {"version": "2.0.0"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropSemverEqualRuleWhenPropDoesNotEqualTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"app": {"version": "2.0.1"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropSemverEqualRuleWhenPropIsNotAValidSemver(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-equal")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"app": {"version": "2.0"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_SEMVER_LESS_THAN

func TestEnabled_ReturnsTrueForPropSemverLessThanRuleWhenPropIsLessThan2_0_0(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"app": {"version": "1.5.1"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropSemverLessThanRuleWhenPropEqualsTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"app": {"version": "2.0.0"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropSemverLessThanRuleWhenPropIsGreaterThanTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-less-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"app": {"version": "2.2.1"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// PROP_SEMVER_GREATER_THAN

func TestEnabled_ReturnsTrueForPropSemverGreaterThanRuleWhenPropIsGreaterThan2_0_0(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"app": {"version": "2.5.1"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabled_ReturnsFalseForPropSemverGreaterThanRuleWhenPropEqualsTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"app": {"version": "2.0.0"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabled_ReturnsFalseForPropSemverEqualRuleWhenPropIsLessThanTheVersion(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.semver-greater-than")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"app": {"version": "0.0.5"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}
