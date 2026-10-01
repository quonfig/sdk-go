// Code generated from integration-test-data/tests/eval/context_precedence.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"

	quonfig "github.com/quonfig/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// returns the correct `flag` value using the global context (1)
func TestContextPrecedence_ReturnsTheCorrectFlagValueUsingTheGlobalContext1(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "verified"}})))
	on, _ := c.FeatureIsOn("mixed.case.property.name", nil)
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}

// returns the correct `flag` value using the global context (2)
func TestContextPrecedence_ReturnsTheCorrectFlagValueUsingTheGlobalContext2(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "?"}})))
	on, _ := c.FeatureIsOn("mixed.case.property.name", nil)
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}

// returns the correct `flag` value when local context clobbers global context (1)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersGlobalContext1(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "?"}})))
	on, _ := c.FeatureIsOn("mixed.case.property.name", contextSet(map[string]map[string]interface{}{"user": {"isHuman": "verified"}}))
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}

// returns the correct `flag` value when local context clobbers global context (2)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersGlobalContext2(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "verified"}})))
	on, _ := c.FeatureIsOn("mixed.case.property.name", contextSet(map[string]map[string]interface{}{"user": {"isHuman": "?"}}))
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}

// returns the correct `flag` value when block context clobbers global context (1)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenBlockContextClobbersGlobalContext1(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "verified"}})))
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "?"}})).FeatureIsOn("mixed.case.property.name")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}

// returns the correct `flag` value when block context clobbers global context (2)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenBlockContextClobbersGlobalContext2(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "?"}})))
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "verified"}})).FeatureIsOn("mixed.case.property.name")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}

// returns the correct `flag` value when local context clobbers block context (1)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersBlockContext1(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "verified"}})).WithContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "?"}})).FeatureIsOn("mixed.case.property.name")
	assert.Equal(t, false, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}

// returns the correct `flag` value when local context clobbers block context (2)
func TestContextPrecedence_ReturnsTheCorrectFlagValueWhenLocalContextClobbersBlockContext2(t *testing.T) {
	c := mustPublicClient(t)
	on, _ := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "?"}})).WithContext(contextSet(map[string]map[string]interface{}{"user": {"isHuman": "verified"}})).FeatureIsOn("mixed.case.property.name")
	assert.Equal(t, true, on, "FeatureIsOn(%q)", "mixed.case.property.name")
}

// returns the correct `get` value using the global context (1)
func TestContextPrecedence_ReturnsTheCorrectGetValueUsingTheGlobalContext1(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})))
	got, ok, err := c.GetStringValue("basic.rule.config", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "override"
	assert.Equal(t, want, got)
}

// returns the correct `get` value using the global context (2)
func TestContextPrecedence_ReturnsTheCorrectGetValueUsingTheGlobalContext2(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@example.com"}})))
	got, ok, err := c.GetStringValue("basic.rule.config", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "default"
	assert.Equal(t, want, got)
}

// returns the correct `get` value when local context clobbers global context (1)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersGlobalContext1(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@example.com"}})))
	got, ok, err := c.GetStringValue("basic.rule.config", contextSet(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "override"
	assert.Equal(t, want, got)
}

// returns the correct `get` value when local context clobbers global context (2)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersGlobalContext2(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})))
	got, ok, err := c.GetStringValue("basic.rule.config", contextSet(map[string]map[string]interface{}{"user": {"email": "test@example.com"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "default"
	assert.Equal(t, want, got)
}

// returns the correct `get` value when block context clobbers global context (1)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenBlockContextClobbersGlobalContext1(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})))
	got, ok, err := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@example.com"}})).GetStringValue("basic.rule.config")
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "default"
	assert.Equal(t, want, got)
}

// returns the correct `get` value when block context clobbers global context (2)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenBlockContextClobbersGlobalContext2(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@example.com"}})))
	got, ok, err := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})).GetStringValue("basic.rule.config")
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "override"
	assert.Equal(t, want, got)
}

// returns the correct `get` value when local context clobbers block context (1)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersBlockContext1(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})).WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@example.com"}})).GetStringValue("basic.rule.config")
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "default"
	assert.Equal(t, want, got)
}

// returns the correct `get` value when local context clobbers block context (2)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextClobbersBlockContext2(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@example.com"}})).WithContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})).GetStringValue("basic.rule.config")
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "override"
	assert.Equal(t, want, got)
}

// returns the correct `get` value when local context replaces the whole global named context (disjoint attributes)
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenLocalContextReplacesTheWholeGlobalNamedContextDisjointAttributes(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})))
	got, ok, err := c.GetStringValue("basic.rule.config", contextSet(map[string]map[string]interface{}{"user": {"plan": "pro"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "default"
	assert.Equal(t, want, got)
}

// returns the correct `get` value when a named context the local context does not mention survives
func TestContextPrecedence_ReturnsTheCorrectGetValueWhenANamedContextTheLocalContextDoesNotMentionSurvives(t *testing.T) {
	c := newPublicClient(t, quonfig.WithGlobalContext(contextSet(map[string]map[string]interface{}{"user": {"email": "test@prefab.cloud"}})))
	got, ok, err := c.GetStringValue("basic.rule.config", contextSet(map[string]map[string]interface{}{"team": {"plan": "pro"}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "basic.rule.config")
	want := "override"
	assert.Equal(t, want, got)
}
