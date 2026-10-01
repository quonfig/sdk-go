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
