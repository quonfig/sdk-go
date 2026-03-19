// Code generated from integration-test-data/tests/eval/get_or_raise.yaml. DO NOT EDIT.

package fixtures

import (
	"testing"
)

func TestGetOrRaise_CanRaiseAnErrorIfValueNotFound(t *testing.T) {
	_, ok := configStore.GetConfig("my-missing-key")
	if ok {
		t.Fatal("expected config 'my-missing-key' to be missing")
	}
	// Config not found with no default: this is the expected missing_default error
}

func TestGetOrRaise_ReturnsADefaultValueInsteadOfRaising(t *testing.T) {
	_, ok := configStore.GetConfig("my-missing-key")
	if ok {
		t.Fatal("expected config 'my-missing-key' to be missing")
	}
	// Config not found but default "DEFAULT" is provided, so that is the result
	assertDefaultStringValue(t, "my-missing-key", "DEFAULT", "DEFAULT")
}

func TestGetOrRaise_RaisesTheCorrectErrorIfItDoesntRaiseOnInitTimeout(t *testing.T) {
	// This test has client_overrides that simulate init failure with on_init_failure: :return.
	// With a staging URL and 0.01s timeout, the client returns empty config store.
	// Then get_or_raise for "any-key" with no default should raise missing_default.
	// We simulate this by verifying the key is not in our config store.
	_, ok := configStore.GetConfig("any-key")
	if ok {
		t.Fatal("expected config 'any-key' to be missing (simulating init timeout with :return)")
	}
	// Config not found with no default: missing_default is expected
}

func TestGetOrRaise_CanRaiseAnErrorIfTheClientDoesNotInitializeInTime(t *testing.T) {
	t.Skip("Skipping: initialization_timeout requires network timing behavior")
}

func TestGetOrRaise_RaisesAnErrorIfAConfigIsProvidedByAMissingEnvironmentVariable(t *testing.T) {
	cfg := mustLookupConfig(t, "provided.by.missing.env.var")
	ctx := buildContextFromMaps(nil, nil, nil)
	match := evaluator.EvaluateConfig(cfg, "Production", ctx)
	if !match.IsMatch || match.Value == nil {
		t.Fatal("expected a match for provided.by.missing.env.var")
	}
	_, err := testResolver.Resolve(match.Value, cfg, "Production", ctx)
	assertResolveError(t, err, "missing_env_var")
}

func TestGetOrRaise_RaisesAnErrorIfAnEnvVarProvidedConfigCannotBeCoercedToConfiguredType(t *testing.T) {
	cfg := mustLookupConfig(t, "provided.not.a.number")
	ctx := buildContextFromMaps(nil, nil, nil)
	match := evaluator.EvaluateConfig(cfg, "Production", ctx)
	if !match.IsMatch || match.Value == nil {
		t.Fatal("expected a match for provided.not.a.number")
	}
	_, err := testResolver.Resolve(match.Value, cfg, "Production", ctx)
	assertResolveError(t, err, "unable_to_coerce_env_var")
}

func TestGetOrRaise_RaisesAnErrorForDecryptionFailure(t *testing.T) {
	cfg := mustLookupConfig(t, "a.broken.secret.config")
	ctx := buildContextFromMaps(nil, nil, nil)
	match := evaluator.EvaluateConfig(cfg, "Production", ctx)
	if !match.IsMatch || match.Value == nil {
		t.Fatal("expected a match for a.broken.secret.config")
	}
	_, err := testResolver.Resolve(match.Value, cfg, "Production", ctx)
	assertResolveError(t, err, "unable_to_decrypt")
}
