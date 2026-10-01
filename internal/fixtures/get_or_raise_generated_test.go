// Code generated from integration-test-data/tests/eval/get_or_raise.yaml. DO NOT EDIT.
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

// get_or_raise can raise an error if value not found
func TestGetOrRaise_GetOrRaiseCanRaiseAnErrorIfValueNotFound(t *testing.T) {
	c := mustPublicClient(t)
	_, ok, err := c.GetStringValue("my-missing-key", nil)
	assert.False(t, ok, "%q must have no value", "my-missing-key")
	require.ErrorIs(t, err, quonfig.ErrNotFound)
}

// get_or_raise returns a default value instead of raising
func TestGetOrRaise_GetOrRaiseReturnsADefaultValueInsteadOfRaising(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("my-missing-key", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// get_or_raise raises the correct error if it doesn't raise on init timeout
func TestGetOrRaise_GetOrRaiseRaisesTheCorrectErrorIfItDoesnTRaiseOnInitTimeout(t *testing.T) {
	c := newInitTimeoutClient(t, 0.01, "https://app.staging-prefab.cloud", "return")
	_, ok, err := c.GetStringValue("any-key", nil)
	assert.False(t, ok, "%q must have no value", "any-key")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "ReturnZeroValue must not surface the init timeout")
}

// get_or_raise can raise an error if the client does not initialize in time
func TestGetOrRaise_GetOrRaiseCanRaiseAnErrorIfTheClientDoesNotInitializeInTime(t *testing.T) {
	c := newInitTimeoutClient(t, 0.01, "https://app.staging-prefab.cloud", "raise")
	_, ok, err := c.GetStringValue("any-key", nil)
	assert.False(t, ok, "%q must have no value", "any-key")
	require.ErrorIs(t, err, quonfig.ErrInitializationTimeout)
}

// raises an error if a config is provided by a missing environment variable
func TestGetOrRaise_RaisesAnErrorIfAConfigIsProvidedByAMissingEnvironmentVariable(t *testing.T) {
	c := mustPublicClient(t)
	_, ok, err := c.GetStringValue("provided.by.missing.env.var", nil)
	assert.False(t, ok, "%q must have no value", "provided.by.missing.env.var")
	require.ErrorIs(t, err, quonfig.ErrMissingEnvVar)
}

// raises an error if an env-var-provided config cannot be coerced to configured type
func TestGetOrRaise_RaisesAnErrorIfAnEnvVarProvidedConfigCannotBeCoercedToConfiguredType(t *testing.T) {
	c := mustPublicClient(t)
	_, ok, err := c.GetIntValue("provided.not.a.number", nil)
	assert.False(t, ok, "%q must have no value", "provided.not.a.number")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}

// raises an error for decryption failure
func TestGetOrRaise_RaisesAnErrorForDecryptionFailure(t *testing.T) {
	c := mustPublicClient(t)
	_, ok, err := c.GetStringValue("a.broken.secret.config", nil)
	assert.False(t, ok, "%q must have no value", "a.broken.secret.config")
	require.ErrorIs(t, err, quonfig.ErrUnableToDecrypt)
}

// raises an error if an env-var-provided duration 30s cannot be coerced
func TestGetOrRaise_RaisesAnErrorIfAnEnvVarProvidedDuration30sCannotBeCoerced(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_30S", "30s")
	c := mustPublicClient(t)
	_, ok, err := c.GetDurationValue("provided.duration.malformed.30s", nil)
	assert.False(t, ok, "%q must have no value", "provided.duration.malformed.30s")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}

// raises an error if an env-var-provided duration PT0.5H cannot be coerced
func TestGetOrRaise_RaisesAnErrorIfAnEnvVarProvidedDurationPT05HCannotBeCoerced(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_PT0_5H", "PT0.5H")
	c := mustPublicClient(t)
	_, ok, err := c.GetDurationValue("provided.duration.malformed.PT0.5H", nil)
	assert.False(t, ok, "%q must have no value", "provided.duration.malformed.PT0.5H")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}

// raises an error if an env-var-provided duration P1DT cannot be coerced
func TestGetOrRaise_RaisesAnErrorIfAnEnvVarProvidedDurationP1DTCannotBeCoerced(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_P1DT", "P1DT")
	c := mustPublicClient(t)
	_, ok, err := c.GetDurationValue("provided.duration.malformed.P1DT", nil)
	assert.False(t, ok, "%q must have no value", "provided.duration.malformed.P1DT")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}

// raises an error if an env-var-provided duration garbage cannot be coerced
func TestGetOrRaise_RaisesAnErrorIfAnEnvVarProvidedDurationGarbageCannotBeCoerced(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_GARBAGE", "garbage")
	c := mustPublicClient(t)
	_, ok, err := c.GetDurationValue("provided.duration.malformed.garbage", nil)
	assert.False(t, ok, "%q must have no value", "provided.duration.malformed.garbage")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}

// raises an error if a stored duration 30s cannot be coerced
func TestGetOrRaise_RaisesAnErrorIfAStoredDuration30sCannotBeCoerced(t *testing.T) {
	c := mustPublicClient(t)
	_, ok, err := c.GetDurationValue("test.duration.malformed.30s", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.30s")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}

// raises an error if a stored duration PT0.5H cannot be coerced
func TestGetOrRaise_RaisesAnErrorIfAStoredDurationPT05HCannotBeCoerced(t *testing.T) {
	c := mustPublicClient(t)
	_, ok, err := c.GetDurationValue("test.duration.malformed.PT0.5H", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.PT0.5H")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}

// raises an error if a stored duration P1DT cannot be coerced
func TestGetOrRaise_RaisesAnErrorIfAStoredDurationP1DTCannotBeCoerced(t *testing.T) {
	c := mustPublicClient(t)
	_, ok, err := c.GetDurationValue("test.duration.malformed.P1DT", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.P1DT")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}

// raises an error if a stored duration garbage cannot be coerced
func TestGetOrRaise_RaisesAnErrorIfAStoredDurationGarbageCannotBeCoerced(t *testing.T) {
	c := mustPublicClient(t)
	_, ok, err := c.GetDurationValue("test.duration.malformed.garbage", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.garbage")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}

// raises an error if a stored duration empty cannot be coerced
func TestGetOrRaise_RaisesAnErrorIfAStoredDurationEmptyCannotBeCoerced(t *testing.T) {
	c := mustPublicClient(t)
	_, ok, err := c.GetDurationValue("test.duration.malformed.empty", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.empty")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
}
