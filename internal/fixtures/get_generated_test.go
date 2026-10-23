// Code generated from integration-test-data/tests/eval/get.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// get returns a found value for key
func TestGet_GetReturnsAFoundValueForKey(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("my-test-key", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "my-test-key")
	want := "my-test-value"
	assert.Equal(t, want, got)
}

// get returns nil if value not found
func TestGet_GetReturnsNilIfValueNotFound(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("my-missing-key", nil)
	assert.False(t, ok, "%q must have no value", "my-missing-key")
	assert.Zero(t, got)
}

// get returns a default for a missing value if a default is given
func TestGet_GetReturnsADefaultForAMissingValueIfADefaultIsGiven(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("my-missing-key", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// get ignores a provided default if the key is found
func TestGet_GetIgnoresAProvidedDefaultIfTheKeyIsFound(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("my-test-key", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "my-test-value"
	assert.Equal(t, want, got)
}

// get can return a double
func TestGet_GetCanReturnADouble(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetFloatValue("my-double-key", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "my-double-key")
	want := 9.95
	assert.Equal(t, want, got)
}

// get can return a string list
func TestGet_GetCanReturnAStringList(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringSliceValue("my-string-list-key", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "my-string-list-key")
	want := []string{"a", "b", "c"}
	assert.Equal(t, want, got)
}

// can return a value provided by an environment variable
func TestGet_CanReturnAValueProvidedByAnEnvironmentVariable(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("prefab.secrets.encryption.key", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "prefab.secrets.encryption.key")
	want := "c87ba22d8662282abe8a0e4651327b579cb64a454ab0f4c170b45b15f049a221"
	assert.Equal(t, want, got)
}

// can return a value provided by an environment variable after type coercion
func TestGet_CanReturnAValueProvidedByAnEnvironmentVariableAfterTypeCoercion(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("provided.a.number", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "provided.a.number")
	want := int64(1234)
	assert.Equal(t, want, got)
}

// can decrypt and return a secret value (with decryption key in in env var)
func TestGet_CanDecryptAndReturnASecretValueWithDecryptionKeyInInEnvVar(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("a.secret.config", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "a.secret.config")
	want := "hello.world"
	assert.Equal(t, want, got)
}

// decrypts a secret whose key config has an environment rule
func TestGet_DecryptsASecretWhoseKeyConfigHasAnEnvironmentRule(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("env.scoped.secret", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "env.scoped.secret")
	want := "hello.world"
	assert.Equal(t, want, got)
}

// duration 200 ms
func TestGet_Duration200Ms(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT0.2S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT0.2S")
	want := time.Duration(200) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration 90S
func TestGet_Duration90S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT90S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT90S")
	want := time.Duration(90000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration 30M
func TestGet_Duration30M(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT30M", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT30M")
	want := time.Duration(1800000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration test.duration.P1DT6H2M1.5S
func TestGet_DurationTestDurationP1DT6H2M15S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.P1DT6H2M1.5S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.P1DT6H2M1.5S")
	want := time.Duration(108121500) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration zero PT0S
func TestGet_DurationZeroPT0S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT0S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT0S")
	want := time.Duration(0) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration zero P0D
func TestGet_DurationZeroP0D(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.P0D", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.P0D")
	want := time.Duration(0) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration days only P2D
func TestGet_DurationDaysOnlyP2D(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.P2D", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.P2D")
	want := time.Duration(172800000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration hours only PT1H
func TestGet_DurationHoursOnlyPT1H(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT1H", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT1H")
	want := time.Duration(3600000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration minutes only PT1M
func TestGet_DurationMinutesOnlyPT1M(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT1M", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT1M")
	want := time.Duration(60000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration seconds only PT1S
func TestGet_DurationSecondsOnlyPT1S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT1S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT1S")
	want := time.Duration(1000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration leading zero PT05S
func TestGet_DurationLeadingZeroPT05S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT05S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT05S")
	want := time.Duration(5000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration hours and minutes PT1H30M
func TestGet_DurationHoursAndMinutesPT1H30M(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT1H30M", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT1H30M")
	want := time.Duration(5400000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration days and hours P1DT2H
func TestGet_DurationDaysAndHoursP1DT2H(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.P1DT2H", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.P1DT2H")
	want := time.Duration(93600000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration one millisecond PT0.001S
func TestGet_DurationOneMillisecondPT0001S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT0.001S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT0.001S")
	want := time.Duration(1) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration magnitude ceiling P36500D
func TestGet_DurationMagnitudeCeilingP36500D(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.P36500D", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.P36500D")
	want := time.Duration(3153600000000) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration rounding PT2.01S
func TestGet_DurationRoundingPT201S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT2.01S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT2.01S")
	want := time.Duration(2010) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration rounding PT1.005S
func TestGet_DurationRoundingPT1005S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT1.005S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT1.005S")
	want := time.Duration(1005) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration rounding half up PT0.0005S
func TestGet_DurationRoundingHalfUpPT00005S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT0.0005S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT0.0005S")
	want := time.Duration(1) * time.Millisecond
	assert.Equal(t, want, got)
}

// duration rounding down PT0.0004S
func TestGet_DurationRoundingDownPT00004S(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("test.duration.PT0.0004S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.duration.PT0.0004S")
	want := time.Duration(0) * time.Millisecond
	assert.Equal(t, want, got)
}

// json test
func TestGet_JsonTest(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetJSONValue("test.json", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.json")
	want := map[string]interface{}{"a": 1, "b": "c"}
	assertJSONValue(t, want, got)
}

// get returns a native json object (not a stringified payload)
func TestGet_GetReturnsANativeJsonObjectNotAStringifiedPayload(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetJSONValue("test.json", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "test.json")
	want := map[string]interface{}{"a": 1, "b": "c"}
	assertJSONValue(t, want, got)
}

// list on left side test (1)
func TestGet_ListOnLeftSideTest1(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("left.hand.list.test", contextSet(map[string]map[string]interface{}{"user": {"name": "james", "aka": []interface{}{"happy", "sleepy"}}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "left.hand.list.test")
	want := "correct"
	assert.Equal(t, want, got)
}

// list on left side test (2)
func TestGet_ListOnLeftSideTest2(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("left.hand.list.test", contextSet(map[string]map[string]interface{}{"user": {"name": "james", "aka": []interface{}{"a", "b"}}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "left.hand.list.test")
	want := "default"
	assert.Equal(t, want, got)
}

// list on left side test opposite (1)
func TestGet_ListOnLeftSideTestOpposite1(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("left.hand.test.opposite", contextSet(map[string]map[string]interface{}{"user": {"name": "james", "aka": []interface{}{"happy", "sleepy"}}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "left.hand.test.opposite")
	want := "default"
	assert.Equal(t, want, got)
}

// list on left side test (3)
func TestGet_ListOnLeftSideTest3(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetStringValue("left.hand.test.opposite", contextSet(map[string]map[string]interface{}{"user": {"name": "james", "aka": []interface{}{"a", "b"}}}))
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "left.hand.test.opposite")
	want := "correct"
	assert.Equal(t, want, got)
}

// env-var-provided duration PT1.5S via get
func TestGet_EnvVarProvidedDurationPT15SViaGet(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_PT1_5S", "PT1.5S")
	c := mustPublicClient(t)
	got, ok, err := c.GetDurationValue("provided.duration.PT1.5S", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "provided.duration.PT1.5S")
	want := time.Duration(1500) * time.Millisecond
	assert.Equal(t, want, got)
}

// stored malformed duration 30s returns the default
func TestGet_StoredMalformedDuration30sReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.30s", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// stored malformed duration 30s with no default returns nil
func TestGet_StoredMalformedDuration30sWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.30s", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.30s")
	assert.Zero(t, got)
}

// stored malformed duration PT0.5H returns the default
func TestGet_StoredMalformedDurationPT05HReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.PT0.5H", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// stored malformed duration PT0.5H with no default returns nil
func TestGet_StoredMalformedDurationPT05HWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.PT0.5H", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.PT0.5H")
	assert.Zero(t, got)
}

// stored malformed duration P1DT returns the default
func TestGet_StoredMalformedDurationP1DTReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.P1DT", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// stored malformed duration P1DT with no default returns nil
func TestGet_StoredMalformedDurationP1DTWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.P1DT", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.P1DT")
	assert.Zero(t, got)
}

// stored malformed duration garbage returns the default
func TestGet_StoredMalformedDurationGarbageReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.garbage", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// stored malformed duration garbage with no default returns nil
func TestGet_StoredMalformedDurationGarbageWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.garbage", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.garbage")
	assert.Zero(t, got)
}

// stored malformed duration empty returns the default
func TestGet_StoredMalformedDurationEmptyReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.empty", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// stored malformed duration empty with no default returns nil
func TestGet_StoredMalformedDurationEmptyWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("test.duration.malformed.empty", nil)
	assert.False(t, ok, "%q must have no value", "test.duration.malformed.empty")
	assert.Zero(t, got)
}

// env-var-provided malformed duration 30s returns the default
func TestGet_EnvVarProvidedMalformedDuration30sReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_30S", "30s")
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("provided.duration.malformed.30s", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// env-var-provided malformed duration 30s with no default returns nil
func TestGet_EnvVarProvidedMalformedDuration30sWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_30S", "30s")
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("provided.duration.malformed.30s", nil)
	assert.False(t, ok, "%q must have no value", "provided.duration.malformed.30s")
	assert.Zero(t, got)
}

// env-var-provided malformed duration PT0.5H returns the default
func TestGet_EnvVarProvidedMalformedDurationPT05HReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_PT0_5H", "PT0.5H")
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("provided.duration.malformed.PT0.5H", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// env-var-provided malformed duration PT0.5H with no default returns nil
func TestGet_EnvVarProvidedMalformedDurationPT05HWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_PT0_5H", "PT0.5H")
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("provided.duration.malformed.PT0.5H", nil)
	assert.False(t, ok, "%q must have no value", "provided.duration.malformed.PT0.5H")
	assert.Zero(t, got)
}

// env-var-provided malformed duration P1DT returns the default
func TestGet_EnvVarProvidedMalformedDurationP1DTReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_P1DT", "P1DT")
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("provided.duration.malformed.P1DT", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// env-var-provided malformed duration P1DT with no default returns nil
func TestGet_EnvVarProvidedMalformedDurationP1DTWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_P1DT", "P1DT")
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("provided.duration.malformed.P1DT", nil)
	assert.False(t, ok, "%q must have no value", "provided.duration.malformed.P1DT")
	assert.Zero(t, got)
}

// env-var-provided malformed duration garbage returns the default
func TestGet_EnvVarProvidedMalformedDurationGarbageReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_GARBAGE", "garbage")
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("provided.duration.malformed.garbage", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// env-var-provided malformed duration garbage with no default returns nil
func TestGet_EnvVarProvidedMalformedDurationGarbageWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_GARBAGE", "garbage")
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("provided.duration.malformed.garbage", nil)
	assert.False(t, ok, "%q must have no value", "provided.duration.malformed.garbage")
	assert.Zero(t, got)
}

// STRING getter on stored int returns the default
func TestGet_STRINGGetterOnStoredIntReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.int", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// STRING getter on stored int with no default returns nil
func TestGet_STRINGGetterOnStoredIntWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.int", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.int")
	assert.Zero(t, got)
}

// STRING getter on stored double returns the default
func TestGet_STRINGGetterOnStoredDoubleReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.double", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// STRING getter on stored double with no default returns nil
func TestGet_STRINGGetterOnStoredDoubleWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.double", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.double")
	assert.Zero(t, got)
}

// STRING getter on stored bool returns the default
func TestGet_STRINGGetterOnStoredBoolReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.bool", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// STRING getter on stored bool with no default returns nil
func TestGet_STRINGGetterOnStoredBoolWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.bool", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.bool")
	assert.Zero(t, got)
}

// STRING getter on stored string_list returns the default
func TestGet_STRINGGetterOnStoredStringListReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.string_list", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// STRING getter on stored string_list with no default returns nil
func TestGet_STRINGGetterOnStoredStringListWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.string_list", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string_list")
	assert.Zero(t, got)
}

// STRING getter on stored json returns the default
func TestGet_STRINGGetterOnStoredJsonReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.json", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// STRING getter on stored json with no default returns nil
func TestGet_STRINGGetterOnStoredJsonWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.json", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.json")
	assert.Zero(t, got)
}

// STRING getter on stored duration returns the default
func TestGet_STRINGGetterOnStoredDurationReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.duration", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// STRING getter on stored duration with no default returns nil
func TestGet_STRINGGetterOnStoredDurationWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("getter.contract.duration", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.duration")
	assert.Zero(t, got)
}

// INT getter on stored string returns the default
func TestGet_INTGetterOnStoredStringReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.string", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored string with no default returns nil
func TestGet_INTGetterOnStoredStringWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.string", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string")
	assert.Zero(t, got)
}

// INT getter on stored double 3.9 returns the default
func TestGet_INTGetterOnStoredDouble39ReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.double", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored double 3.9 with no default returns nil
func TestGet_INTGetterOnStoredDouble39WithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.double", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.double")
	assert.Zero(t, got)
}

// INT getter on stored bool returns the default
func TestGet_INTGetterOnStoredBoolReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.bool", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored bool with no default returns nil
func TestGet_INTGetterOnStoredBoolWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.bool", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.bool")
	assert.Zero(t, got)
}

// INT getter on stored string_list returns the default
func TestGet_INTGetterOnStoredStringListReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.string_list", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored string_list with no default returns nil
func TestGet_INTGetterOnStoredStringListWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.string_list", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string_list")
	assert.Zero(t, got)
}

// INT getter on stored json returns the default
func TestGet_INTGetterOnStoredJsonReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.json", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored json with no default returns nil
func TestGet_INTGetterOnStoredJsonWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.json", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.json")
	assert.Zero(t, got)
}

// INT getter on stored duration returns the default
func TestGet_INTGetterOnStoredDurationReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.duration", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored duration with no default returns nil
func TestGet_INTGetterOnStoredDurationWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.duration", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.duration")
	assert.Zero(t, got)
}

// DOUBLE getter on stored string returns the default
func TestGet_DOUBLEGetterOnStoredStringReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.string", nil)
	if !ok {
		got = 7.5
	}
	want := 7.5
	assert.Equal(t, want, got)
}

// DOUBLE getter on stored string with no default returns nil
func TestGet_DOUBLEGetterOnStoredStringWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.string", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string")
	assert.Zero(t, got)
}

// DOUBLE getter on stored bool returns the default
func TestGet_DOUBLEGetterOnStoredBoolReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.bool", nil)
	if !ok {
		got = 7.5
	}
	want := 7.5
	assert.Equal(t, want, got)
}

// DOUBLE getter on stored bool with no default returns nil
func TestGet_DOUBLEGetterOnStoredBoolWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.bool", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.bool")
	assert.Zero(t, got)
}

// DOUBLE getter on stored string_list returns the default
func TestGet_DOUBLEGetterOnStoredStringListReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.string_list", nil)
	if !ok {
		got = 7.5
	}
	want := 7.5
	assert.Equal(t, want, got)
}

// DOUBLE getter on stored string_list with no default returns nil
func TestGet_DOUBLEGetterOnStoredStringListWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.string_list", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string_list")
	assert.Zero(t, got)
}

// DOUBLE getter on stored json returns the default
func TestGet_DOUBLEGetterOnStoredJsonReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.json", nil)
	if !ok {
		got = 7.5
	}
	want := 7.5
	assert.Equal(t, want, got)
}

// DOUBLE getter on stored json with no default returns nil
func TestGet_DOUBLEGetterOnStoredJsonWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.json", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.json")
	assert.Zero(t, got)
}

// DOUBLE getter on stored duration returns the default
func TestGet_DOUBLEGetterOnStoredDurationReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.duration", nil)
	if !ok {
		got = 7.5
	}
	want := 7.5
	assert.Equal(t, want, got)
}

// DOUBLE getter on stored duration with no default returns nil
func TestGet_DOUBLEGetterOnStoredDurationWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.duration", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.duration")
	assert.Zero(t, got)
}

// BOOLEAN getter on stored string returns the default
func TestGet_BOOLEANGetterOnStoredStringReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.string", nil)
	if !ok {
		got = false
	}
	want := false
	assert.Equal(t, want, got)
}

// BOOLEAN getter on stored string with no default returns nil
func TestGet_BOOLEANGetterOnStoredStringWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.string", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string")
	assert.Zero(t, got)
}

// BOOLEAN getter on stored int returns the default
func TestGet_BOOLEANGetterOnStoredIntReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.int", nil)
	if !ok {
		got = false
	}
	want := false
	assert.Equal(t, want, got)
}

// BOOLEAN getter on stored int with no default returns nil
func TestGet_BOOLEANGetterOnStoredIntWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.int", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.int")
	assert.Zero(t, got)
}

// BOOLEAN getter on stored double returns the default
func TestGet_BOOLEANGetterOnStoredDoubleReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.double", nil)
	if !ok {
		got = false
	}
	want := false
	assert.Equal(t, want, got)
}

// BOOLEAN getter on stored double with no default returns nil
func TestGet_BOOLEANGetterOnStoredDoubleWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.double", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.double")
	assert.Zero(t, got)
}

// BOOLEAN getter on stored string_list returns the default
func TestGet_BOOLEANGetterOnStoredStringListReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.string_list", nil)
	if !ok {
		got = false
	}
	want := false
	assert.Equal(t, want, got)
}

// BOOLEAN getter on stored string_list with no default returns nil
func TestGet_BOOLEANGetterOnStoredStringListWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.string_list", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string_list")
	assert.Zero(t, got)
}

// BOOLEAN getter on stored json returns the default
func TestGet_BOOLEANGetterOnStoredJsonReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.json", nil)
	if !ok {
		got = false
	}
	want := false
	assert.Equal(t, want, got)
}

// BOOLEAN getter on stored json with no default returns nil
func TestGet_BOOLEANGetterOnStoredJsonWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.json", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.json")
	assert.Zero(t, got)
}

// BOOLEAN getter on stored duration returns the default
func TestGet_BOOLEANGetterOnStoredDurationReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.duration", nil)
	if !ok {
		got = false
	}
	want := false
	assert.Equal(t, want, got)
}

// BOOLEAN getter on stored duration with no default returns nil
func TestGet_BOOLEANGetterOnStoredDurationWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.duration", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.duration")
	assert.Zero(t, got)
}

// STRING_LIST getter on stored string returns the default
func TestGet_STRINGLISTGetterOnStoredStringReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.string", nil)
	if !ok {
		got = []string{"DEFAULT"}
	}
	want := []string{"DEFAULT"}
	assert.Equal(t, want, got)
}

// STRING_LIST getter on stored string with no default returns nil
func TestGet_STRINGLISTGetterOnStoredStringWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.string", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string")
	assert.Zero(t, got)
}

// STRING_LIST getter on stored int returns the default
func TestGet_STRINGLISTGetterOnStoredIntReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.int", nil)
	if !ok {
		got = []string{"DEFAULT"}
	}
	want := []string{"DEFAULT"}
	assert.Equal(t, want, got)
}

// STRING_LIST getter on stored int with no default returns nil
func TestGet_STRINGLISTGetterOnStoredIntWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.int", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.int")
	assert.Zero(t, got)
}

// STRING_LIST getter on stored double returns the default
func TestGet_STRINGLISTGetterOnStoredDoubleReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.double", nil)
	if !ok {
		got = []string{"DEFAULT"}
	}
	want := []string{"DEFAULT"}
	assert.Equal(t, want, got)
}

// STRING_LIST getter on stored double with no default returns nil
func TestGet_STRINGLISTGetterOnStoredDoubleWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.double", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.double")
	assert.Zero(t, got)
}

// STRING_LIST getter on stored bool returns the default
func TestGet_STRINGLISTGetterOnStoredBoolReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.bool", nil)
	if !ok {
		got = []string{"DEFAULT"}
	}
	want := []string{"DEFAULT"}
	assert.Equal(t, want, got)
}

// STRING_LIST getter on stored bool with no default returns nil
func TestGet_STRINGLISTGetterOnStoredBoolWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.bool", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.bool")
	assert.Zero(t, got)
}

// STRING_LIST getter on stored json returns the default
func TestGet_STRINGLISTGetterOnStoredJsonReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.json", nil)
	if !ok {
		got = []string{"DEFAULT"}
	}
	want := []string{"DEFAULT"}
	assert.Equal(t, want, got)
}

// STRING_LIST getter on stored json with no default returns nil
func TestGet_STRINGLISTGetterOnStoredJsonWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.json", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.json")
	assert.Zero(t, got)
}

// STRING_LIST getter on stored duration returns the default
func TestGet_STRINGLISTGetterOnStoredDurationReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.duration", nil)
	if !ok {
		got = []string{"DEFAULT"}
	}
	want := []string{"DEFAULT"}
	assert.Equal(t, want, got)
}

// STRING_LIST getter on stored duration with no default returns nil
func TestGet_STRINGLISTGetterOnStoredDurationWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringSliceValue("getter.contract.duration", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.duration")
	assert.Zero(t, got)
}

// JSON getter on stored string returns the default
func TestGet_JSONGetterOnStoredStringReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.string", nil)
	if !ok {
		got = map[string]interface{}{"default": true}
	}
	want := map[string]interface{}{"default": true}
	assertJSONValue(t, want, got)
}

// JSON getter on stored string with no default returns nil
func TestGet_JSONGetterOnStoredStringWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.string", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string")
	assert.Zero(t, got)
}

// JSON getter on stored int returns the default
func TestGet_JSONGetterOnStoredIntReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.int", nil)
	if !ok {
		got = map[string]interface{}{"default": true}
	}
	want := map[string]interface{}{"default": true}
	assertJSONValue(t, want, got)
}

// JSON getter on stored int with no default returns nil
func TestGet_JSONGetterOnStoredIntWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.int", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.int")
	assert.Zero(t, got)
}

// JSON getter on stored double returns the default
func TestGet_JSONGetterOnStoredDoubleReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.double", nil)
	if !ok {
		got = map[string]interface{}{"default": true}
	}
	want := map[string]interface{}{"default": true}
	assertJSONValue(t, want, got)
}

// JSON getter on stored double with no default returns nil
func TestGet_JSONGetterOnStoredDoubleWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.double", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.double")
	assert.Zero(t, got)
}

// JSON getter on stored bool returns the default
func TestGet_JSONGetterOnStoredBoolReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.bool", nil)
	if !ok {
		got = map[string]interface{}{"default": true}
	}
	want := map[string]interface{}{"default": true}
	assertJSONValue(t, want, got)
}

// JSON getter on stored bool with no default returns nil
func TestGet_JSONGetterOnStoredBoolWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.bool", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.bool")
	assert.Zero(t, got)
}

// JSON getter on stored string_list returns the default
func TestGet_JSONGetterOnStoredStringListReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.string_list", nil)
	if !ok {
		got = map[string]interface{}{"default": true}
	}
	want := map[string]interface{}{"default": true}
	assertJSONValue(t, want, got)
}

// JSON getter on stored string_list with no default returns nil
func TestGet_JSONGetterOnStoredStringListWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.string_list", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string_list")
	assert.Zero(t, got)
}

// JSON getter on stored duration returns the default
func TestGet_JSONGetterOnStoredDurationReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.duration", nil)
	if !ok {
		got = map[string]interface{}{"default": true}
	}
	want := map[string]interface{}{"default": true}
	assertJSONValue(t, want, got)
}

// JSON getter on stored duration with no default returns nil
func TestGet_JSONGetterOnStoredDurationWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetJSONValue("getter.contract.duration", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.duration")
	assert.Zero(t, got)
}

// DURATION getter on stored string returns the default
func TestGet_DURATIONGetterOnStoredStringReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.string", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// DURATION getter on stored string with no default returns nil
func TestGet_DURATIONGetterOnStoredStringWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.string", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string")
	assert.Zero(t, got)
}

// DURATION getter on stored int returns the default
func TestGet_DURATIONGetterOnStoredIntReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.int", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// DURATION getter on stored int with no default returns nil
func TestGet_DURATIONGetterOnStoredIntWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.int", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.int")
	assert.Zero(t, got)
}

// DURATION getter on stored double returns the default
func TestGet_DURATIONGetterOnStoredDoubleReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.double", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// DURATION getter on stored double with no default returns nil
func TestGet_DURATIONGetterOnStoredDoubleWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.double", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.double")
	assert.Zero(t, got)
}

// DURATION getter on stored bool returns the default
func TestGet_DURATIONGetterOnStoredBoolReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.bool", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// DURATION getter on stored bool with no default returns nil
func TestGet_DURATIONGetterOnStoredBoolWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.bool", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.bool")
	assert.Zero(t, got)
}

// DURATION getter on stored string_list returns the default
func TestGet_DURATIONGetterOnStoredStringListReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.string_list", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// DURATION getter on stored string_list with no default returns nil
func TestGet_DURATIONGetterOnStoredStringListWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.string_list", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string_list")
	assert.Zero(t, got)
}

// DURATION getter on stored json returns the default
func TestGet_DURATIONGetterOnStoredJsonReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.json", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// DURATION getter on stored json with no default returns nil
func TestGet_DURATIONGetterOnStoredJsonWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.json", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.json")
	assert.Zero(t, got)
}

// BOOLEAN getter on stored string "false" returns the default
func TestGet_BOOLEANGetterOnStoredStringFalseReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.string.false", nil)
	if !ok {
		got = false
	}
	want := false
	assert.Equal(t, want, got)
}

// BOOLEAN getter on stored string "false" with no default returns nil
func TestGet_BOOLEANGetterOnStoredStringFalseWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.string.false", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string.false")
	assert.Zero(t, got)
}

// BOOLEAN getter on stored string "false" returns a true default
func TestGet_BOOLEANGetterOnStoredStringFalseReturnsATrueDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.string.false", nil)
	if !ok {
		got = true
	}
	want := true
	assert.Equal(t, want, got)
}

// BOOLEAN getter on stored string "true" returns the default
func TestGet_BOOLEANGetterOnStoredStringTrueReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.string.true", nil)
	if !ok {
		got = false
	}
	want := false
	assert.Equal(t, want, got)
}

// BOOLEAN getter on stored string "true" with no default returns nil
func TestGet_BOOLEANGetterOnStoredStringTrueWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.string.true", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string.true")
	assert.Zero(t, got)
}

// INT getter on stored string "3" returns the default
func TestGet_INTGetterOnStoredString3ReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.string.number", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored string "3" with no default returns nil
func TestGet_INTGetterOnStoredString3WithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.string.number", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string.number")
	assert.Zero(t, got)
}

// DOUBLE getter on stored string "3" returns the default
func TestGet_DOUBLEGetterOnStoredString3ReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.string.number", nil)
	if !ok {
		got = 7.5
	}
	want := 7.5
	assert.Equal(t, want, got)
}

// DOUBLE getter on stored string "3" with no default returns nil
func TestGet_DOUBLEGetterOnStoredString3WithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.string.number", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string.number")
	assert.Zero(t, got)
}

// DURATION getter on stored string "PT1S" returns the default
func TestGet_DURATIONGetterOnStoredStringPT1SReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.string.duration", nil)
	if !ok {
		got = time.Duration(7000) * time.Millisecond
	}
	want := time.Duration(7000) * time.Millisecond
	assert.Equal(t, want, got)
}

// DURATION getter on stored string "PT1S" with no default returns nil
func TestGet_DURATIONGetterOnStoredStringPT1SWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetDurationValue("getter.contract.string.duration", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.string.duration")
	assert.Zero(t, got)
}

// DOUBLE get on stored int 30 returns 30.0
func TestGet_DOUBLEGetOnStoredInt30Returns300(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetFloatValue("getter.contract.int", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "getter.contract.int")
	want := 30.0
	assert.Equal(t, want, got)
}

// INT get on stored whole double 3.0 returns 3
func TestGet_INTGetOnStoredWholeDouble30Returns3(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, err := c.GetIntValue("getter.contract.double.whole", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "getter.contract.double.whole")
	want := int64(3)
	assert.Equal(t, want, got)
}

// DOUBLE getter on stored int 30 ignores the default
func TestGet_DOUBLEGetterOnStoredInt30IgnoresTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.int", nil)
	if !ok {
		got = 7.5
	}
	want := 30.0
	assert.Equal(t, want, got)
}

// INT getter on stored whole double 3.0 ignores the default
func TestGet_INTGetterOnStoredWholeDouble30IgnoresTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.double.whole", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(3)
	assert.Equal(t, want, got)
}

// INT getter on stored double 1e300 returns the default
func TestGet_INTGetterOnStoredDouble1e300ReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.double.huge", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored double 1e300 with no default returns nil
func TestGet_INTGetterOnStoredDouble1e300WithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.double.huge", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.double.huge")
	assert.Zero(t, got)
}

// INT getter on stored double 2^53 (outside the safe int range) returns the default
func TestGet_INTGetterOnStoredDouble253OutsideTheSafeIntRangeReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.double.unsafe", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored double 2^53 (outside the safe int range) with no default returns nil
func TestGet_INTGetterOnStoredDouble253OutsideTheSafeIntRangeWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.double.unsafe", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.double.unsafe")
	assert.Zero(t, got)
}

// INT getter on stored malformed int "twelve" returns the default
func TestGet_INTGetterOnStoredMalformedIntTwelveReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.int.malformed", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on stored malformed int "twelve" with no default returns nil
func TestGet_INTGetterOnStoredMalformedIntTwelveWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.int.malformed", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.int.malformed")
	assert.Zero(t, got)
}

// DOUBLE getter on stored malformed double "abc" returns the default
func TestGet_DOUBLEGetterOnStoredMalformedDoubleAbcReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.double.malformed", nil)
	if !ok {
		got = 7.5
	}
	want := 7.5
	assert.Equal(t, want, got)
}

// DOUBLE getter on stored malformed double "abc" with no default returns nil
func TestGet_DOUBLEGetterOnStoredMalformedDoubleAbcWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.double.malformed", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.double.malformed")
	assert.Zero(t, got)
}

// INT getter on env-var-provided malformed int "twelve" returns the default
func TestGet_INTGetterOnEnvVarProvidedMalformedIntTwelveReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_GETTER_INT", "twelve")
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.provided.int.malformed", nil)
	if !ok {
		got = int64(7)
	}
	want := int64(7)
	assert.Equal(t, want, got)
}

// INT getter on env-var-provided malformed int "twelve" with no default returns nil
func TestGet_INTGetterOnEnvVarProvidedMalformedIntTwelveWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_GETTER_INT", "twelve")
	c := mustPublicClient(t)
	got, ok, _ := c.GetIntValue("getter.contract.provided.int.malformed", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.provided.int.malformed")
	assert.Zero(t, got)
}

// DOUBLE getter on env-var-provided malformed double "abc" returns the default
func TestGet_DOUBLEGetterOnEnvVarProvidedMalformedDoubleAbcReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_GETTER_DOUBLE", "abc")
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.provided.double.malformed", nil)
	if !ok {
		got = 7.5
	}
	want := 7.5
	assert.Equal(t, want, got)
}

// DOUBLE getter on env-var-provided malformed double "abc" with no default returns nil
func TestGet_DOUBLEGetterOnEnvVarProvidedMalformedDoubleAbcWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_GETTER_DOUBLE", "abc")
	c := mustPublicClient(t)
	got, ok, _ := c.GetFloatValue("getter.contract.provided.double.malformed", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.provided.double.malformed")
	assert.Zero(t, got)
}

// STRING getter on a missing env var returns the default
func TestGet_STRINGGetterOnAMissingEnvVarReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("provided.by.missing.env.var", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// STRING getter on a missing env var with no default returns nil
func TestGet_STRINGGetterOnAMissingEnvVarWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("provided.by.missing.env.var", nil)
	assert.False(t, ok, "%q must have no value", "provided.by.missing.env.var")
	assert.Zero(t, got)
}

// STRING getter on a decryption failure returns the default
func TestGet_STRINGGetterOnADecryptionFailureReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("a.broken.secret.config", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// STRING getter on a decryption failure with no default returns nil
func TestGet_STRINGGetterOnADecryptionFailureWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetStringValue("a.broken.secret.config", nil)
	assert.False(t, ok, "%q must have no value", "a.broken.secret.config")
	assert.Zero(t, got)
}

// BOOLEAN getter on a missing bool env var returns the default
func TestGet_BOOLEANGetterOnAMissingBoolEnvVarReturnsTheDefault(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.provided.bool.missing", nil)
	if !ok {
		got = true
	}
	want := true
	assert.Equal(t, want, got)
}

// BOOLEAN getter on a missing bool env var with no default returns nil
func TestGet_BOOLEANGetterOnAMissingBoolEnvVarWithNoDefaultReturnsNil(t *testing.T) {
	c := mustPublicClient(t)
	got, ok, _ := c.GetBoolValue("getter.contract.provided.bool.missing", nil)
	assert.False(t, ok, "%q must have no value", "getter.contract.provided.bool.missing")
	assert.Zero(t, got)
}

// BOOLEAN getter on env-var-provided bool "false" returns false
func TestGet_BOOLEANGetterOnEnvVarProvidedBoolFalseReturnsFalse(t *testing.T) {
	t.Setenv("QUONFIG_ITD_GETTER_BOOL", "false")
	c := mustPublicClient(t)
	got, ok, err := c.GetBoolValue("getter.contract.provided.bool", nil)
	require.NoError(t, err)
	require.True(t, ok, "%q found no value", "getter.contract.provided.bool")
	want := false
	assert.Equal(t, want, got)
}
