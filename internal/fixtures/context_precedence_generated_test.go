// Code generated from integration-test-data/tests/eval/context_precedence.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// returns the correct `flag` value using the global context (1)
func TestContextPrecedence_ReturnsTheCorrectFlagValueUsingTheGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"isHuman": "verified"}}, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns the correct `flag` value using the global context (2)
func TestContextPrecedence_ReturnsTheCorrectFlagValueUsingTheGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"isHuman": "?"}}, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns the correct `flag` value when local context clobbers global context (1)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"isHuman": "?"}}, nil, map[string]map[string]interface{}{"user": {"isHuman": "verified"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns the correct `flag` value when local context clobbers global context (2)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"isHuman": "verified"}}, nil, map[string]map[string]interface{}{"user": {"isHuman": "?"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns the correct `flag` value when block context clobbers global context (1)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenBlockContextClobbersGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"isHuman": "verified"}}, map[string]map[string]interface{}{"user": {"isHuman": "?"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns the correct `flag` value when block context clobbers global context (2)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenBlockContextClobbersGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"isHuman": "?"}}, map[string]map[string]interface{}{"user": {"isHuman": "verified"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns the correct `flag` value when local context clobbers block context (1)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersBlockContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"isHuman": "verified"}}, map[string]map[string]interface{}{"user": {"isHuman": "?"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// returns the correct `flag` value when local context clobbers block context (2)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersBlockContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"isHuman": "?"}}, map[string]map[string]interface{}{"user": {"isHuman": "verified"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// returns the correct `get` value using the global context (1)
func TestContextPrecedence_ReturnsTheCorrectGetValueUsingTheGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}}, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "override")
}

// returns the correct `get` value using the global context (2)
func TestContextPrecedence_ReturnsTheCorrectGetValueUsingTheGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"email": "test@example.com"}}, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

// returns the correct `get` value when local context clobbers global context (1)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"email": "test@example.com"}}, nil, map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "override")
}

// returns the correct `get` value when local context clobbers global context (2)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}}, nil, map[string]map[string]interface{}{"user": {"email": "test@example.com"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

// returns the correct `get` value when block context clobbers global context (1)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenBlockContextClobbersGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}}, map[string]map[string]interface{}{"user": {"email": "test@example.com"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

// returns the correct `get` value when block context clobbers global context (2)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenBlockContextClobbersGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(map[string]map[string]interface{}{"user": {"email": "test@example.com"}}, map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "override")
}

// returns the correct `get` value when local context clobbers block context (1)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersBlockContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}}, map[string]map[string]interface{}{"user": {"email": "test@example.com"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

// returns the correct `get` value when local context clobbers block context (2)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersBlockContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "test@example.com"}}, map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "override")
}
