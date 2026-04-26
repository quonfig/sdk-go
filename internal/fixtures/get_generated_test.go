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
	cfg := mustLookupConfig(t, "my-missing-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "DEFAULT")
}

// get ignores a provided default if the key is found
func TestGet_GetIgnoresAProvidedDefaultIfTheKeyIsFound(t *testing.T) {
	cfg := mustLookupConfig(t, "my-test-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "my-test-value")
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
	cfg := mustLookupConfig(t, "test.duration.PT0.2S")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 200)
}

// duration 90S
func TestGet_Duration90S(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.PT90S")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 90000)
}

// duration 1.5M
func TestGet_Duration15M(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.PT1.5M")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 90000)
}

// duration 0.5H
func TestGet_Duration05H(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.PT0.5H")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 1800000)
}

// duration test.duration.P1DT6H2M1.5S
func TestGet_DurationTestDurationP1DT6H2M15S(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.P1DT6H2M1.5S")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 108121500)
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
