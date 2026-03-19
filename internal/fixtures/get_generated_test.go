// Code generated from integration-test-data/tests/eval/get.yaml. DO NOT EDIT.

package fixtures

import (
	"testing"
)

func TestGet_GetReturnsAFoundValueForKey(t *testing.T) {
	cfg := mustLookupConfig(t, "my-test-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "my-test-value")
}

func TestGet_GetReturnsNilIfValueNotFound(t *testing.T) {
	_, ok := configStore.GetConfig("my-missing-key")
	if ok {
		t.Fatal("expected config 'my-missing-key' to be missing")
	}
	// on_no_default: 2 means return nil for missing keys
	// Config not found, expected nil -- pass
}

func TestGet_GetReturnsADefaultForAMissingValueIfADefaultIsGiven(t *testing.T) {
	_, ok := configStore.GetConfig("my-missing-key")
	if ok {
		t.Fatal("expected config 'my-missing-key' to be missing")
	}
	// Config not found, default "DEFAULT" is the expected answer
	assertDefaultStringValue(t, "my-missing-key", "DEFAULT", "DEFAULT")
}

func TestGet_GetIgnoresAProvidedDefaultIfTheKeyIsFound(t *testing.T) {
	cfg := mustLookupConfig(t, "my-test-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	// Even though a default "DEFAULT" is provided, the key is found so we get the real value
	assertStringValue(t, match, "my-test-value")
}

func TestGet_GetCanReturnADouble(t *testing.T) {
	cfg := mustLookupConfig(t, "my-double-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDoubleValue(t, match, 9.95)
}

func TestGet_GetCanReturnAStringList(t *testing.T) {
	cfg := mustLookupConfig(t, "my-string-list-key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringListValue(t, match, []string{"a", "b", "c"})
}

func TestGet_CanReturnAnOverrideBasedOnTheDefaultContext(t *testing.T) {
	cfg := mustLookupConfig(t, "my-overridden-key")
	// This config uses prefab-api-key.* criteria which are injected server-side
	if shouldSkipAPIContextTest(cfg) {
		t.Skip("Skipping: test requires API-injected context (prefab-api-key.*) not available in local eval")
	}
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "overridden")
}

func TestGet_CanReturnAValueProvidedByAnEnvironmentVariable(t *testing.T) {
	cfg := mustLookupConfig(t, "prefab.secrets.encryption.key")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "c87ba22d8662282abe8a0e4651327b579cb64a454ab0f4c170b45b15f049a221")
}

func TestGet_CanReturnAValueProvidedByAnEnvironmentVariableAfterTypeCoercion(t *testing.T) {
	cfg := mustLookupConfig(t, "provided.a.number")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertIntValue(t, match, 1234)
}

func TestGet_CanDecryptAndReturnASecretValue(t *testing.T) {
	cfg := mustLookupConfig(t, "a.secret.config")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "hello.world")
}

func TestGet_Duration200Ms(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.PT0.2S")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 200)
}

func TestGet_Duration90S(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.PT90S")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 90000)
}

func TestGet_Duration1Point5M(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.PT1.5M")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 90000)
}

func TestGet_Duration0Point5H(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.PT0.5H")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 1800000)
}

func TestGet_DurationP1DT6H2M1Point5S(t *testing.T) {
	cfg := mustLookupConfig(t, "test.duration.P1DT6H2M1.5S")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertDurationMillis(t, match, 108121500)
}

func TestGet_JSONTest(t *testing.T) {
	cfg := mustLookupConfig(t, "test.json")
	ctx := buildContextFromMaps(nil, nil, nil)
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertJSONValue(t, match, map[string]interface{}{
		"a": 1,
		"b": "c",
	})
}

func TestGet_ListOnLeftSideTest1(t *testing.T) {
	cfg := mustLookupConfig(t, "left.hand.list.test")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {
			"name": "james",
			"aka":  []interface{}{"happy", "sleepy"},
		},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "correct")
}

func TestGet_ListOnLeftSideTest2(t *testing.T) {
	cfg := mustLookupConfig(t, "left.hand.list.test")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {
			"name": "james",
			"aka":  []interface{}{"a", "b"},
		},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

func TestGet_ListOnLeftSideTestOpposite1(t *testing.T) {
	cfg := mustLookupConfig(t, "left.hand.test.opposite")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {
			"name": "james",
			"aka":  []interface{}{"happy", "sleepy"},
		},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "default")
}

func TestGet_ListOnLeftSideTest3(t *testing.T) {
	cfg := mustLookupConfig(t, "left.hand.test.opposite")
	ctx := buildContextFromMaps(nil, nil, map[string]map[string]interface{}{
		"user": {
			"name": "james",
			"aka":  []interface{}{"a", "b"},
		},
	})
	match, err := evaluateAndResolve(t, cfg, ctx)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}
	assertStringValue(t, match, "correct")
}
