// Code generated from integration-test-data/tests/eval/context_precedence.yaml. DO NOT EDIT.

package fixtures

import (
	"testing"
)

// enabled tests

func TestContextPrecedence_ReturnsTheCorrectFlagValueUsingTheGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"isHuman": "verified"},
		}, nil, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestContextPrecedence_ReturnsTheCorrectFlagValueUsingTheGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"isHuman": "?"},
		}, nil, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"isHuman": "?"},
		}, nil,
		map[string]map[string]interface{}{
			"user": {"isHuman": "verified"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"isHuman": "verified"},
		}, nil,
		map[string]map[string]interface{}{
			"user": {"isHuman": "?"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenBlockContextClobbersGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"isHuman": "verified"},
		},
		map[string]map[string]interface{}{
			"user": {"isHuman": "?"},
		}, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenBlockContextClobbersGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"isHuman": "?"},
		},
		map[string]map[string]interface{}{
			"user": {"isHuman": "verified"},
		}, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersBlockContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(nil,
		map[string]map[string]interface{}{
			"user": {"isHuman": "verified"},
		},
		map[string]map[string]interface{}{
			"user": {"isHuman": "?"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersBlockContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "mixed.case.property.name")
	ctx := buildContextFromMaps(nil,
		map[string]map[string]interface{}{
			"user": {"isHuman": "?"},
		},
		map[string]map[string]interface{}{
			"user": {"isHuman": "verified"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// get tests

func TestContextPrecedence_ReturnsTheCorrectGetValueUsingTheGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"email": "test@prefab.cloud"},
		}, nil, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "override")
}

func TestContextPrecedence_ReturnsTheCorrectGetValueUsingTheGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"email": "test@example.com"},
		}, nil, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

func TestContextPrecedence_ReturnsTheCorrectGetValueUsingTheGlobalContextAndApiContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config.with.api.conditional")
	if shouldSkipAPIContextTest(cfg) {
		t.Skip("Skipping: test requires API-injected context (prefab-api-key.*) not available in local eval")
	}
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"email": "test@prefab.cloud"},
		}, nil, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "override")
}

func TestContextPrecedence_ReturnsTheCorrectGetValueUsingTheGlobalContextAndApiContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config.with.api.conditional")
	if shouldSkipAPIContextTest(cfg) {
		t.Skip("Skipping: test requires API-injected context (prefab-api-key.*) not available in local eval")
	}
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"email": "test@example.com"},
		}, nil, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "api-override")
}

func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"email": "test@example.com"},
		}, nil,
		map[string]map[string]interface{}{
			"user": {"email": "test@prefab.cloud"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "override")
}

func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"email": "test@prefab.cloud"},
		}, nil,
		map[string]map[string]interface{}{
			"user": {"email": "test@example.com"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

func TestContextPrecedence_ReturnsTheCorrectGetValueWhenBlockContextClobbersGlobalContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"email": "test@prefab.cloud"},
		},
		map[string]map[string]interface{}{
			"user": {"email": "test@example.com"},
		}, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

func TestContextPrecedence_ReturnsTheCorrectGetValueWhenBlockContextClobbersGlobalContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(
		map[string]map[string]interface{}{
			"user": {"email": "test@example.com"},
		},
		map[string]map[string]interface{}{
			"user": {"email": "test@prefab.cloud"},
		}, nil,
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "override")
}

func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersBlockContext1(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(nil,
		map[string]map[string]interface{}{
			"user": {"email": "test@prefab.cloud"},
		},
		map[string]map[string]interface{}{
			"user": {"email": "test@example.com"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersBlockContext2(t *testing.T) {
	cfg := mustLookupConfig(t, "basic.rule.config")
	ctx := buildContextFromMaps(nil,
		map[string]map[string]interface{}{
			"user": {"email": "test@example.com"},
		},
		map[string]map[string]interface{}{
			"user": {"email": "test@prefab.cloud"},
		},
	)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "override")
}
