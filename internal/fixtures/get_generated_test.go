// Code generated from integration-test-data/tests/eval/get.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// get returns a found value for key
func TestGet_GetReturnsAFoundValueForKey(t *testing.T) {
	cfg := mustLookupConfig(t, "my-test-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "my-test-value")
}

// get returns nil if value not found
func TestGet_GetReturnsNilIfValueNotFound(t *testing.T) {
	cfg := mustLookupConfig(t, "my-missing-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}

// get returns a default for a missing value if a default is given
func TestGet_GetReturnsADefaultForAMissingValueIfADefaultIsGiven(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	assertGetWithDefault(t, "my-missing-key", ctx, "DEFAULT", "DEFAULT")
}

// get ignores a provided default if the key is found
func TestGet_GetIgnoresAProvidedDefaultIfTheKeyIsFound(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	assertGetWithDefault(t, "my-test-key", ctx, "DEFAULT", "my-test-value")
}

// get can return a double
func TestGet_GetCanReturnADouble(t *testing.T) {
	cfg := mustLookupConfig(t, "my-double-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDoubleValue(t, match, 9.95)
}

// get can return a string list
func TestGet_GetCanReturnAStringList(t *testing.T) {
	cfg := mustLookupConfig(t, "my-string-list-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringListValue(t, match, []string{"a", "b", "c"})
}

// can return a value provided by an environment variable
func TestGet_CanReturnAValueProvidedByAnEnvironmentVariable(t *testing.T) {
	cfg := mustLookupConfig(t, "prefab.secrets.encryption.key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "c87ba22d8662282abe8a0e4651327b579cb64a454ab0f4c170b45b15f049a221")
}

// can return a value provided by an environment variable after type coercion
func TestGet_CanReturnAValueProvidedByAnEnvironmentVariableAfterTypeCoercion(t *testing.T) {
	cfg := mustLookupConfig(t, "provided.a.number")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 1234)
}

// can decrypt and return a secret value (with decryption key in in env var)
func TestGet_CanDecryptAndReturnASecretValueWithDecryptionKeyInInEnvVar(t *testing.T) {
	cfg := mustLookupConfig(t, "a.secret.config")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "hello.world")
}

// duration 200 ms
func TestGet_Duration200Ms(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT0.2S", ctx, 200)
}

// duration 90S
func TestGet_Duration90S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT90S", ctx, 90000)
}

// duration 30M
func TestGet_Duration30M(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT30M", ctx, 1800000)
}

// duration test.duration.P1DT6H2M1.5S
func TestGet_DurationTestDurationP1DT6H2M15S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.P1DT6H2M1.5S", ctx, 108121500)
}

// duration zero PT0S
func TestGet_DurationZeroPT0S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT0S", ctx, 0)
}

// duration zero P0D
func TestGet_DurationZeroP0D(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.P0D", ctx, 0)
}

// duration days only P2D
func TestGet_DurationDaysOnlyP2D(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.P2D", ctx, 172800000)
}

// duration hours only PT1H
func TestGet_DurationHoursOnlyPT1H(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT1H", ctx, 3600000)
}

// duration minutes only PT1M
func TestGet_DurationMinutesOnlyPT1M(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT1M", ctx, 60000)
}

// duration seconds only PT1S
func TestGet_DurationSecondsOnlyPT1S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT1S", ctx, 1000)
}

// duration leading zero PT05S
func TestGet_DurationLeadingZeroPT05S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT05S", ctx, 5000)
}

// duration hours and minutes PT1H30M
func TestGet_DurationHoursAndMinutesPT1H30M(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT1H30M", ctx, 5400000)
}

// duration days and hours P1DT2H
func TestGet_DurationDaysAndHoursP1DT2H(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.P1DT2H", ctx, 93600000)
}

// duration one millisecond PT0.001S
func TestGet_DurationOneMillisecondPT0001S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT0.001S", ctx, 1)
}

// duration magnitude ceiling P36500D
func TestGet_DurationMagnitudeCeilingP36500D(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.P36500D", ctx, 3153600000000)
}

// duration rounding PT2.01S
func TestGet_DurationRoundingPT201S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT2.01S", ctx, 2010)
}

// duration rounding PT1.005S
func TestGet_DurationRoundingPT1005S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT1.005S", ctx, 1005)
}

// duration rounding half up PT0.0005S
func TestGet_DurationRoundingHalfUpPT00005S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT0.0005S", ctx, 1)
}

// duration rounding down PT0.0004S
func TestGet_DurationRoundingDownPT00004S(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.PT0.0004S", ctx, 0)
}

// json test
func TestGet_JsonTest(t *testing.T) {
	cfg := mustLookupConfig(t, "test.json")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertJSONValue(t, match, map[string]interface{}{"a": 1, "b": "c"})
}

// get returns a native json object (not a stringified payload)
func TestGet_GetReturnsANativeJsonObjectNotAStringifiedPayload(t *testing.T) {
	cfg := mustLookupConfig(t, "test.json")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertJSONValue(t, match, map[string]interface{}{"a": 1, "b": "c"})
}

// list on left side test (1)
func TestGet_ListOnLeftSideTest1(t *testing.T) {
	cfg := mustLookupConfig(t, "left.hand.list.test")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"name": "james", "aka": []interface{}{"happy", "sleepy"}}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "correct")
}

// list on left side test (2)
func TestGet_ListOnLeftSideTest2(t *testing.T) {
	cfg := mustLookupConfig(t, "left.hand.list.test")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"name": "james", "aka": []interface{}{"a", "b"}}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

// list on left side test opposite (1)
func TestGet_ListOnLeftSideTestOpposite1(t *testing.T) {
	cfg := mustLookupConfig(t, "left.hand.test.opposite")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"name": "james", "aka": []interface{}{"happy", "sleepy"}}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

// list on left side test (3)
func TestGet_ListOnLeftSideTest3(t *testing.T) {
	cfg := mustLookupConfig(t, "left.hand.test.opposite")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{"user": {"name": "james", "aka": []interface{}{"a", "b"}}})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "correct")
}

// env-var-provided duration PT1.5S via get
func TestGet_EnvVarProvidedDurationPT15SViaGet(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_PT1_5S", "PT1.5S");
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "provided.duration.PT1.5S", ctx, 1500)
}

// stored malformed duration 30s returns the default
func TestGet_StoredMalformedDuration30sReturnsTheDefault(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.malformed.30s", ctx, 7000)
}

// stored malformed duration 30s with no default returns nil
func TestGet_StoredMalformedDuration30sWithNoDefaultReturnsNil(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.malformed.30s")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}

// stored malformed duration PT0.5H returns the default
func TestGet_StoredMalformedDurationPT05HReturnsTheDefault(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.malformed.PT0.5H", ctx, 7000)
}

// stored malformed duration PT0.5H with no default returns nil
func TestGet_StoredMalformedDurationPT05HWithNoDefaultReturnsNil(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.malformed.PT0.5H")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}

// stored malformed duration P1DT returns the default
func TestGet_StoredMalformedDurationP1DTReturnsTheDefault(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.malformed.P1DT", ctx, 7000)
}

// stored malformed duration P1DT with no default returns nil
func TestGet_StoredMalformedDurationP1DTWithNoDefaultReturnsNil(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.malformed.P1DT")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}

// stored malformed duration garbage returns the default
func TestGet_StoredMalformedDurationGarbageReturnsTheDefault(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.malformed.garbage", ctx, 7000)
}

// stored malformed duration garbage with no default returns nil
func TestGet_StoredMalformedDurationGarbageWithNoDefaultReturnsNil(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.malformed.garbage")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}

// stored malformed duration empty returns the default
func TestGet_StoredMalformedDurationEmptyReturnsTheDefault(t *testing.T) {
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "test.duration.malformed.empty", ctx, 7000)
}

// stored malformed duration empty with no default returns nil
func TestGet_StoredMalformedDurationEmptyWithNoDefaultReturnsNil(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.malformed.empty")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}

// env-var-provided malformed duration 30s returns the default
func TestGet_EnvVarProvidedMalformedDuration30sReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_30S", "30s");
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "provided.duration.malformed.30s", ctx, 7000)
}

// env-var-provided malformed duration 30s with no default returns nil
func TestGet_EnvVarProvidedMalformedDuration30sWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_30S", "30s");
	cfg := mustLookupConfig(t, "provided.duration.malformed.30s")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}

// env-var-provided malformed duration PT0.5H returns the default
func TestGet_EnvVarProvidedMalformedDurationPT05HReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_PT0_5H", "PT0.5H");
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "provided.duration.malformed.PT0.5H", ctx, 7000)
}

// env-var-provided malformed duration PT0.5H with no default returns nil
func TestGet_EnvVarProvidedMalformedDurationPT05HWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_PT0_5H", "PT0.5H");
	cfg := mustLookupConfig(t, "provided.duration.malformed.PT0.5H")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}

// env-var-provided malformed duration P1DT returns the default
func TestGet_EnvVarProvidedMalformedDurationP1DTReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_P1DT", "P1DT");
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "provided.duration.malformed.P1DT", ctx, 7000)
}

// env-var-provided malformed duration P1DT with no default returns nil
func TestGet_EnvVarProvidedMalformedDurationP1DTWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_P1DT", "P1DT");
	cfg := mustLookupConfig(t, "provided.duration.malformed.P1DT")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}

// env-var-provided malformed duration garbage returns the default
func TestGet_EnvVarProvidedMalformedDurationGarbageReturnsTheDefault(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_GARBAGE", "garbage");
	ctx := buildPublicContext(nil, nil, nil)
	assertDurationMillis(t, "provided.duration.malformed.garbage", ctx, 7000)
}

// env-var-provided malformed duration garbage with no default returns nil
func TestGet_EnvVarProvidedMalformedDurationGarbageWithNoDefaultReturnsNil(t *testing.T) {
	t.Setenv("QUONFIG_ITD_DURATION_GARBAGE", "garbage");
	cfg := mustLookupConfig(t, "provided.duration.malformed.garbage")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertNilValue(t, match)
}
