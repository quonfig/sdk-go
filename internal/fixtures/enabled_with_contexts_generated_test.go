// Code generated from integration-test-data/tests/eval/enabled_with_contexts.yaml. DO NOT EDIT.

package fixtures

import (
	"testing"
)

func TestEnabledWithContexts_ReturnsTrueFromGlobalContext(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"":     {"domain": "prefab.cloud"},
		"user": {"key": "michael"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabledWithContexts_ReturnsFalseDueToLocalContextOverride(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil,
		map[string]map[string]interface{}{
			"":     {"domain": "prefab.cloud"},
			"user": {"key": "michael"},
		},
		map[string]map[string]interface{}{
			"user": {"key": "james"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabledWithContexts_ReturnsFalseForUntouchedScopeContext(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"":     {"domain": "example.com"},
		"user": {"key": "nobody"},
	}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabledWithContexts_ReturnsFalseDueToPartialScopeContextOverrideOfUserKey(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil,
		map[string]map[string]interface{}{
			"":     {"domain": "example.com"},
			"user": {"key": "nobody"},
		},
		map[string]map[string]interface{}{
			"user": {"key": "michael"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabledWithContexts_ReturnsFalseDueToPartialScopeContextOverrideOfDomain(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil,
		map[string]map[string]interface{}{
			"":     {"domain": "example.com"},
			"user": {"key": "nobody"},
		},
		map[string]map[string]interface{}{
			"": {"key": "prefab.cloud"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabledWithContexts_ReturnsTrueDueToFullScopeContextOverrideOfUserKeyAndDomain(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.in-seg.segment-and")
	ctx := buildContextFromMaps(nil,
		map[string]map[string]interface{}{
			"":     {"domain": "example.com"},
			"user": {"key": "nobody"},
		},
		map[string]map[string]interface{}{
			"user": {"key": "michael"},
			"":     {"domain": "prefab.cloud"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestEnabledWithContexts_ReturnsFalseForRuleWithDifferentCaseOnContextPropertyName(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"IsHuman": "verified"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestEnabledWithContexts_ReturnsTrueForMatchingCaseOnContextPropertyName(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {"isHuman": "verified"},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}
