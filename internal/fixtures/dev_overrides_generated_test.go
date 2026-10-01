// Code generated from integration-test-data/tests/eval/dev_overrides.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// override fires when quonfig-user.email matches
func TestDevOverrides_OverrideFiresWhenQuonfigUserEmailMatches(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"quonfig-user": {"email": "bob@foo.com"}})).FeatureIsOn("feature-flag.dev-override")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.dev-override")
}

// override does not fire when attribute absent (prod simulation)
func TestDevOverrides_OverrideDoesNotFireWhenAttributeAbsentProdSimulation(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "bob@foo.com"}})).FeatureIsOn("feature-flag.dev-override")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "feature-flag.dev-override")
}

// override matches any email in IS_ONE_OF list
func TestDevOverrides_OverrideMatchesAnyEmailInISONEOFList(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"quonfig-user": {"email": "alice@foo.com"}})).FeatureIsOn("feature-flag.dev-override.multi-email")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.dev-override.multi-email")
}

// override beats customer rule by priority
func TestDevOverrides_OverrideBeatsCustomerRuleByPriority(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"quonfig-user": {"email": "bob@foo.com"}, "user": {"country": "DE"}})).FeatureIsOn("feature-flag.dev-override.priority")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "feature-flag.dev-override.priority")
}
