// Code generated from integration-test-data/tests/eval/get_or_raise.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// get_or_raise can raise an error if value not found
func TestGetOrRaise_GetOrRaiseCanRaiseAnErrorIfValueNotFound(t *testing.T) {
	_, ok := configStore.GetConfig("my-missing-key")
	if ok {
		t.Fatalf("expected config %q to be missing for missing_default case", "my-missing-key")
	}
}

// get_or_raise returns a default value instead of raising
func TestGetOrRaise_GetOrRaiseReturnsADefaultValueInsteadOfRaising(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	assertGetWithDefault(t, "my-missing-key", ctx, "DEFAULT", "DEFAULT")
}

// get_or_raise raises the correct error if it doesn't raise on init timeout
func TestGetOrRaise_GetOrRaiseRaisesTheCorrectErrorIfItDoesnTRaiseOnInitTimeout(t *testing.T) {
	assertClientConstructionMissingDefault(t, "any-key", 0.01, "https://app.staging-prefab.cloud", "return", "get_or_raise")
}

// get_or_raise can raise an error if the client does not initialize in time
func TestGetOrRaise_GetOrRaiseCanRaiseAnErrorIfTheClientDoesNotInitializeInTime(t *testing.T) {
	assertInitializationTimeoutError(t, "any-key", 0.01, "https://app.staging-prefab.cloud", "raise")
}

// raises an error if a config is provided by a missing environment variable
func TestGetOrRaise_RaisesAnErrorIfAConfigIsProvidedByAMissingEnvironmentVariable(t *testing.T) {
	cfg := mustLookupConfig(t, "provided.by.missing.env.var")
	ctx := buildContextFromMaps(nil, nil, nil)
	match := evaluator.EvaluateConfig(cfg, "Production", ctx)
	if !match.IsMatch || match.Value == nil {
		t.Fatalf("expected a match for %q", "provided.by.missing.env.var")
	}
	_, err := testResolver.Resolve(match.Value, cfg, "Production", ctx)
	assertResolveError(t, err, "missing_env_var")
}

// raises an error if an env-var-provided config cannot be coerced to configured type
func TestGetOrRaise_RaisesAnErrorIfAnEnvVarProvidedConfigCannotBeCoercedToConfiguredType(t *testing.T) {
	cfg := mustLookupConfig(t, "provided.not.a.number")
	ctx := buildContextFromMaps(nil, nil, nil)
	match := evaluator.EvaluateConfig(cfg, "Production", ctx)
	if !match.IsMatch || match.Value == nil {
		t.Fatalf("expected a match for %q", "provided.not.a.number")
	}
	_, err := testResolver.Resolve(match.Value, cfg, "Production", ctx)
	assertResolveError(t, err, "unable_to_coerce_env_var")
}

// raises an error for decryption failure
func TestGetOrRaise_RaisesAnErrorForDecryptionFailure(t *testing.T) {
	cfg := mustLookupConfig(t, "a.broken.secret.config")
	ctx := buildContextFromMaps(nil, nil, nil)
	match := evaluator.EvaluateConfig(cfg, "Production", ctx)
	if !match.IsMatch || match.Value == nil {
		t.Fatalf("expected a match for %q", "a.broken.secret.config")
	}
	_, err := testResolver.Resolve(match.Value, cfg, "Production", ctx)
	assertResolveError(t, err, "unable_to_decrypt")
}
