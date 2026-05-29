// Code generated from integration-test-data/tests/eval/delivery_environment.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	quonfig "github.com/quonfig/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// singular environment override wins over default when env not pinned
func TestDeliveryEnvironment_SingularEnvironmentOverrideWinsOverDefaultWhenEnvNotPinned(t *testing.T) {
	const envelopeJSON = "{\"meta\":{\"version\":\"v1\",\"environment\":\"development\"},\"configs\":[{\"id\":\"c-env\",\"key\":\"flag.env-scoped\",\"type\":\"bool\",\"valueType\":\"bool\",\"sendToClientSdk\":false,\"default\":{\"rules\":[{\"criteria\":[{\"operator\":\"ALWAYS_TRUE\"}],\"value\":{\"type\":\"bool\",\"value\":true}}]},\"environment\":{\"id\":\"development\",\"rules\":[{\"criteria\":[{\"operator\":\"ALWAYS_TRUE\"}],\"value\":{\"type\":\"bool\",\"value\":false}}]}}]}"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/configs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "\"v1\"")
		_, _ = w.Write([]byte(envelopeJSON))
	}))
	defer server.Close()

	client, err := quonfig.NewClient(
		quonfig.WithAPIKey("sdk-test"),
		quonfig.WithAPIURLs([]string{server.URL}),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithInitTimeout(5*time.Second),
	)
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetBoolValue("flag.env-scoped", nil)
	require.NoError(t, err)
	require.True(t, ok, "expected config %q to be present from wire envelope", "flag.env-scoped")
	assert.Equal(t, false, val, "delivery-wire env override: expected %v for %q", false, "flag.env-scoped")
}

// explicit environment pin is ignored in delivery mode (meta.environment authoritative)
func TestDeliveryEnvironment_ExplicitEnvironmentPinIsIgnoredInDeliveryModeMetaEnvironmentAuthoritative(t *testing.T) {
	const envelopeJSON = "{\"meta\":{\"version\":\"v1\",\"environment\":\"development\"},\"configs\":[{\"id\":\"c-env\",\"key\":\"flag.env-scoped\",\"type\":\"bool\",\"valueType\":\"bool\",\"sendToClientSdk\":false,\"default\":{\"rules\":[{\"criteria\":[{\"operator\":\"ALWAYS_TRUE\"}],\"value\":{\"type\":\"bool\",\"value\":true}}]},\"environment\":{\"id\":\"development\",\"rules\":[{\"criteria\":[{\"operator\":\"ALWAYS_TRUE\"}],\"value\":{\"type\":\"bool\",\"value\":false}}]}}]}"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/configs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "\"v1\"")
		_, _ = w.Write([]byte(envelopeJSON))
	}))
	defer server.Close()

	client, err := quonfig.NewClient(
		quonfig.WithAPIKey("sdk-test"),
		quonfig.WithAPIURLs([]string{server.URL}),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithInitTimeout(5*time.Second),
		quonfig.WithEnvironment("staging"),
	)
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetBoolValue("flag.env-scoped", nil)
	require.NoError(t, err)
	require.True(t, ok, "expected config %q to be present from wire envelope", "flag.env-scoped")
	assert.Equal(t, false, val, "delivery-wire env override: expected %v for %q", false, "flag.env-scoped")
}

// config without environment block falls back to default in delivery mode
func TestDeliveryEnvironment_ConfigWithoutEnvironmentBlockFallsBackToDefaultInDeliveryMode(t *testing.T) {
	const envelopeJSON = "{\"meta\":{\"version\":\"v1\",\"environment\":\"development\"},\"configs\":[{\"id\":\"c-def\",\"key\":\"flag.default-only\",\"type\":\"bool\",\"valueType\":\"bool\",\"sendToClientSdk\":false,\"default\":{\"rules\":[{\"criteria\":[{\"operator\":\"ALWAYS_TRUE\"}],\"value\":{\"type\":\"bool\",\"value\":true}}]}}]}"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/configs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", "\"v1\"")
		_, _ = w.Write([]byte(envelopeJSON))
	}))
	defer server.Close()

	client, err := quonfig.NewClient(
		quonfig.WithAPIKey("sdk-test"),
		quonfig.WithAPIURLs([]string{server.URL}),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithInitTimeout(5*time.Second),
	)
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetBoolValue("flag.default-only", nil)
	require.NoError(t, err)
	require.True(t, ok, "expected config %q to be present from wire envelope", "flag.default-only")
	assert.Equal(t, true, val, "delivery-wire env override: expected %v for %q", true, "flag.default-only")
}
