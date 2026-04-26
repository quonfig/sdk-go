// Code generated from integration-test-data/tests/eval/dev_overrides.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// override fires when quonfig-user.email matches
func TestDevOverrides_OverrideFiresWhenQuonfigUserEmailMatches(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.dev-override")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"quonfig-user": {"email": "bob@foo.com"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// override does not fire when attribute absent (prod simulation)
func TestDevOverrides_OverrideDoesNotFireWhenAttributeAbsentProdSimulation(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.dev-override")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"user": {"email": "bob@foo.com"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, false)
}

// override matches any email in IS_ONE_OF list
func TestDevOverrides_OverrideMatchesAnyEmailInISONEOFList(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.dev-override.multi-email")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"quonfig-user": {"email": "alice@foo.com"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}

// override beats customer rule by priority
func TestDevOverrides_OverrideBeatsCustomerRuleByPriority(t *testing.T) {
	cfg := mustLookupConfig(t, "feature-flag.dev-override.priority")
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{"quonfig-user": {"email": "bob@foo.com"}, "user": {"country": "DE"}}, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertEnabledValue(t, match, true)
}
