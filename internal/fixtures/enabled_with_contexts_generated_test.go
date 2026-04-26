// Code generated from integration-test-data/tests/eval/enabled_with_contexts.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// returns true from global context
func TestEnabledWithContexts_ReturnsTrueFromGlobalContext(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"": {"domain": "prefab.cloud"}, "user": {"key": "michael"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false due to local context override
func TestEnabledWithContexts_ReturnsFalseDueToLocalContextOverride(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"": {"domain": "prefab.cloud"}, "user": {"key": "michael"}}, map[string]map[string]interface{}{"user": {"key": "james"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false for untouched scope context
func TestEnabledWithContexts_ReturnsFalseForUntouchedScopeContext(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"": {"domain": "example.com"}, "user": {"key": "nobody"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false due to partial scope context override of user.key
func TestEnabledWithContexts_ReturnsFalseDueToPartialScopeContextOverrideOfUserKey(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"": {"domain": "example.com"}, "user": {"key": "nobody"}}, map[string]map[string]interface{}{"user": {"key": "michael"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns false due to partial scope context override of domain
func TestEnabledWithContexts_ReturnsFalseDueToPartialScopeContextOverrideOfDomain(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"": {"domain": "example.com"}, "user": {"key": "nobody"}}, map[string]map[string]interface{}{"": {"key": "prefab.cloud"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true due to full scope context override of user.key and domain
func TestEnabledWithContexts_ReturnsTrueDueToFullScopeContextOverrideOfUserKeyAndDomain(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"": {"domain": "example.com"}, "user": {"key": "nobody"}}, map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "prefab.cloud"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns false for rule with different case on context property name
func TestEnabledWithContexts_ReturnsFalseForRuleWithDifferentCaseOnContextPropertyName(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"IsHuman": "verified"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns true for matching case on context property name
func TestEnabledWithContexts_ReturnsTrueForMatchingCaseOnContextPropertyName(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"isHuman": "verified"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}
