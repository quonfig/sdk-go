// Code generated from integration-test-data/tests/eval/enabled_with_contexts.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// returns true from global context
func TestEnabledWithContexts_ReturnsTrueFromGlobalContext(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"": {"domain": "prefab.cloud"}, "user": {"key": "michael"}})).FeatureIsOn("feature-flag.in-seg.segment-and")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns false due to local context override
func TestEnabledWithContexts_ReturnsFalseDueToLocalContextOverride(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"": {"domain": "prefab.cloud"}, "user": {"key": "michael"}})).WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "james"}})).FeatureIsOn("feature-flag.in-seg.segment-and")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns false for untouched scope context
func TestEnabledWithContexts_ReturnsFalseForUntouchedScopeContext(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"": {"domain": "example.com"}, "user": {"key": "nobody"}})).FeatureIsOn("feature-flag.in-seg.segment-and")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns false due to partial scope context override of user.key
func TestEnabledWithContexts_ReturnsFalseDueToPartialScopeContextOverrideOfUserKey(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"": {"domain": "example.com"}, "user": {"key": "nobody"}})).WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}})).FeatureIsOn("feature-flag.in-seg.segment-and")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns false due to partial scope context override of domain
func TestEnabledWithContexts_ReturnsFalseDueToPartialScopeContextOverrideOfDomain(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"": {"domain": "example.com"}, "user": {"key": "nobody"}})).WithContext(contextSet(map[string]map[string]interface{}{"": {"domain": "prefab.cloud"}})).FeatureIsOn("feature-flag.in-seg.segment-and")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns true due to local override of domain when scope user.key already matches
func TestEnabledWithContexts_ReturnsTrueDueToLocalOverrideOfDomainWhenScopeUserKeyAlreadyMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"": {"domain": "example.com"}, "user": {"key": "michael"}})).WithContext(contextSet(map[string]map[string]interface{}{"": {"domain": "prefab.cloud"}})).FeatureIsOn("feature-flag.in-seg.segment-and")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns true due to full scope context override of user.key and domain
func TestEnabledWithContexts_ReturnsTrueDueToFullScopeContextOverrideOfUserKeyAndDomain(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"": {"domain": "example.com"}, "user": {"key": "nobody"}})).WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}, "": {"domain": "prefab.cloud"}})).FeatureIsOn("feature-flag.in-seg.segment-and")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.in-seg.segment-and")
}

// returns false for rule with different case on context property name
func TestEnabledWithContexts_ReturnsFalseForRuleWithDifferentCaseOnContextPropertyName(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("mixed.case.property.name", contextSet(map[string]map[string]interface{}{"user": {"IsHuman": "verified"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}

// returns true for matching case on context property name
func TestEnabledWithContexts_ReturnsTrueForMatchingCaseOnContextPropertyName(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.FeatureIsOn("mixed.case.property.name", contextSet(map[string]map[string]interface{}{"user": {"isHuman": "verified"}}))
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}
