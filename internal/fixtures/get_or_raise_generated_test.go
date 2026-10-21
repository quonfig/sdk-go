// Code generated from integration-test-data/tests/eval/get_or_raise.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// init fails by default when every delivery leg refuses the connection
func TestGetOrRaise_InitFailsByDefaultWhenEveryDeliveryLegRefusesTheConnection(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "", []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, 0, 500)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure default = fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unavailable, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unavailable"), initErr.Reason)
	}
	assert.Less(t, elapsed, 5000*time.Millisecond, "init must raise within 5000ms")
}

// init with on_init_failure fail raises when every delivery leg refuses the connection
func TestGetOrRaise_InitWithOnInitFailureFailRaisesWhenEveryDeliveryLegRefusesTheConnection(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, 0, 500)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unavailable, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unavailable"), initErr.Reason)
	}
	assert.Less(t, elapsed, 5000*time.Millisecond, "init must raise within 5000ms")
}

// init with on_init_failure fail raises the init timeout when every delivery leg hangs
func TestGetOrRaise_InitWithOnInitFailureFailRaisesTheInitTimeoutWhenEveryDeliveryLegHangs(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", []string{"http://192.0.2.1:8001", "http://192.0.2.1:8002"}, 0, 200)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.ErrorIs(t, err, quonfig.ErrInitializationTimeout)
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("timeout"), initErr.Reason)
	}
	assert.Less(t, elapsed, 2000*time.Millisecond, "init must raise within 2000ms")
}

// init with on_init_failure fail raises at once when every delivery leg answers 401
func TestGetOrRaise_InitWithOnInitFailureFailRaisesAtOnceWhenEveryDeliveryLegAnswers401(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", []string{}, 401, 8000)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unauthorized, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unauthorized"), initErr.Reason)
	}
	assert.Less(t, elapsed, 4000*time.Millisecond, "init must raise within 4000ms")
}

// get_or_raise returns the default after init continues past refused delivery legs
func TestGetOrRaise_GetOrRaiseReturnsTheDefaultAfterInitContinuesPastRefusedDeliveryLegs(t *testing.T) {
	c, err := newInitFailureClient(t, "continue", []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, 0, 500)
	require.NoError(t, err, "on_init_failure continue: init must return")
	require.NotNil(t, c)
	got, ok, _ := c.GetStringValue("any-key", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// get_or_raise raises missing_default after init continues past refused delivery legs
func TestGetOrRaise_GetOrRaiseRaisesMissingDefaultAfterInitContinuesPastRefusedDeliveryLegs(t *testing.T) {
	c, err := newInitFailureClient(t, "continue", []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, 0, 500)
	require.NoError(t, err, "on_init_failure continue: init must return")
	require.NotNil(t, c)
	_, ok, err := c.GetStringValue("any-key", nil)
	assert.False(t, ok, "%q must have no value", "any-key")
	require.ErrorIs(t, err, quonfig.ErrNotFound)
}

// deprecated on_init_failure raise behaves as fail when every delivery leg refuses the connection
func TestGetOrRaise_DeprecatedOnInitFailureRaiseBehavesAsFailWhenEveryDeliveryLegRefusesTheConnection(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "raise", []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, 0, 500)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure raise)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unavailable, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unavailable"), initErr.Reason)
	}
	assert.Less(t, elapsed, 5000*time.Millisecond, "init must raise within 5000ms")
}

// deprecated on_init_failure return behaves as continue when every delivery leg refuses the connection
func TestGetOrRaise_DeprecatedOnInitFailureReturnBehavesAsContinueWhenEveryDeliveryLegRefusesTheConnection(t *testing.T) {
	c, err := newInitFailureClient(t, "return", []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, 0, 500)
	require.NoError(t, err, "on_init_failure continue: init must return")
	require.NotNil(t, c)
	_, ok, err := c.GetStringValue("any-key", nil)
	assert.False(t, ok, "%q must have no value", "any-key")
	require.ErrorIs(t, err, quonfig.ErrNotFound)
}

// init with on_init_failure fail raises at once when every delivery leg answers 404
func TestGetOrRaise_InitWithOnInitFailureFailRaisesAtOnceWhenEveryDeliveryLegAnswers404(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", []string{}, 404, 8000)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unauthorized, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unauthorized"), initErr.Reason)
	}
	assert.Less(t, elapsed, 4000*time.Millisecond, "init must raise within 4000ms")
}

// init with on_init_failure fail raises at once when every delivery leg answers 410
func TestGetOrRaise_InitWithOnInitFailureFailRaisesAtOnceWhenEveryDeliveryLegAnswers410(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", []string{}, 410, 8000)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unauthorized, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unauthorized"), initErr.Reason)
	}
	assert.Less(t, elapsed, 4000*time.Millisecond, "init must raise within 4000ms")
}

// init with on_init_failure fail raises unavailable when only the primary answers 401 and the secondary 503
func TestGetOrRaise_InitWithOnInitFailureFailRaisesUnavailableWhenOnlyThePrimaryAnswers401AndTheSecondary503(t *testing.T) {
	var legURLs []string
	for _, status := range []int{401, 503} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		t.Cleanup(srv.Close)
		legURLs = append(legURLs, srv.URL)
	}
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", legURLs, 0, 1500)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unavailable, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unavailable"), initErr.Reason)
	}
	assert.Less(t, elapsed, 6000*time.Millisecond, "init must raise within 6000ms")
}

// init with init_timeout_ms 0 and on_init_failure fail raises after one attempt when every delivery leg refuses the connection
func TestGetOrRaise_InitWithInitTimeoutMs0AndOnInitFailureFailRaisesAfterOneAttemptWhenEveryDeliveryLegRefusesTheConnection(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, 0, 0)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unavailable, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unavailable"), initErr.Reason)
	}
	assert.Less(t, elapsed, 5000*time.Millisecond, "init must raise within 5000ms")
}

// get_or_raise returns the default after init with init_timeout_ms 0 continues past refused delivery legs
func TestGetOrRaise_GetOrRaiseReturnsTheDefaultAfterInitWithInitTimeoutMs0ContinuesPastRefusedDeliveryLegs(t *testing.T) {
	c, err := newInitFailureClient(t, "continue", []string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, 0, 0)
	require.NoError(t, err, "on_init_failure continue: init must return")
	require.NotNil(t, c)
	got, ok, _ := c.GetStringValue("any-key", nil)
	if !ok {
		got = "DEFAULT"
	}
	want := "DEFAULT"
	assert.Equal(t, want, got)
}

// init with on_init_failure fail raises the init timeout when every delivery leg hangs past its per-leg abort
func TestGetOrRaise_InitWithOnInitFailureFailRaisesTheInitTimeoutWhenEveryDeliveryLegHangsPastItsPerLegAbort(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", []string{"http://192.0.2.1:8001", "http://192.0.2.1:8002"}, 0, 10000)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.ErrorIs(t, err, quonfig.ErrInitializationTimeout)
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("timeout"), initErr.Reason)
	}
	assert.Less(t, elapsed, 14000*time.Millisecond, "init must raise within 14000ms")
}

// init error never carries the userinfo of a delivery URL
func TestGetOrRaise_InitErrorNeverCarriesTheUserinfoOfADeliveryURL(t *testing.T) {
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", []string{"http://qfguser:qfgsecret@127.0.0.1:1", "http://qfguser:qfgsecret@127.0.0.1:2"}, 0, 500)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unavailable, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unavailable"), initErr.Reason)
	}
	assert.Less(t, elapsed, 5000*time.Millisecond, "init must raise within 5000ms")
	for e := err; e != nil; e = errors.Unwrap(e) {
		assert.NotContains(t, e.Error(), "qfgsecret")
		assert.NotContains(t, e.Error(), "qfguser")
	}
}

// init error never carries an upstream response body
func TestGetOrRaise_InitErrorNeverCarriesAnUpstreamResponseBody(t *testing.T) {
	var legURLs []string
	for _, status := range []int{503, 503} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte("upstream exploded: qfg-upstream-body-marker"))
		}))
		t.Cleanup(srv.Close)
		legURLs = append(legURLs, srv.URL)
	}
	start := time.Now()
	c, err := newInitFailureClient(t, "fail", legURLs, 0, 500)
	elapsed := time.Since(start)
	require.Error(t, err, "init must fail (on_init_failure fail)")
	assert.Nil(t, c, "on_init_failure fail returns no client")
	assert.NotErrorIs(t, err, quonfig.ErrInitializationTimeout, "reason unavailable, not the init timeout")
	var initErr *quonfig.InitError
	if assert.True(t, errors.As(err, &initErr), "init error is a *quonfig.InitError: %v", err) {
		assert.Equal(t, quonfig.InitErrorReason("unavailable"), initErr.Reason)
	}
	assert.Less(t, elapsed, 5000*time.Millisecond, "init must raise within 5000ms")
	for e := err; e != nil; e = errors.Unwrap(e) {
		assert.NotContains(t, e.Error(), "qfg-upstream-body-marker")
	}
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

// coercion error for an int env var names the variable and never its value
func TestGetOrRaise_CoercionErrorForAnIntEnvVarNamesTheVariableAndNeverItsValue(t *testing.T) {
	t.Setenv("QUONFIG_ITD_SECRET_ENV", "postgres://admin:hunter2@db")
	c := mustPublicClient(t)
	_, ok, err := c.GetIntValue("provided.secret.not.an.int", nil)
	assert.False(t, ok, "%q must have no value", "provided.secret.not.an.int")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
	assert.Contains(t, err.Error(), "environment variable \"QUONFIG_ITD_SECRET_ENV\" cannot be converted to int for config \"provided.secret.not.an.int\"")
	assert.NotContains(t, err.Error(), "hunter2")
	assert.NotContains(t, err.Error(), "postgres://")
}

// coercion error for a double env var names the variable and never its value
func TestGetOrRaise_CoercionErrorForADoubleEnvVarNamesTheVariableAndNeverItsValue(t *testing.T) {
	t.Setenv("QUONFIG_ITD_SECRET_ENV", "postgres://admin:hunter2@db")
	c := mustPublicClient(t)
	_, ok, err := c.GetFloatValue("provided.secret.not.a.double", nil)
	assert.False(t, ok, "%q must have no value", "provided.secret.not.a.double")
	require.ErrorIs(t, err, quonfig.ErrUnableToCoerce)
	assert.Contains(t, err.Error(), "environment variable \"QUONFIG_ITD_SECRET_ENV\" cannot be converted to double for config \"provided.secret.not.a.double\"")
	assert.NotContains(t, err.Error(), "hunter2")
	assert.NotContains(t, err.Error(), "postgres://")
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
