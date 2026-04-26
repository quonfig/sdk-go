// Code generated from integration-test-data/tests/eval/datadir_environment.yaml. DO NOT EDIT.
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

// datadir with environment option gets environment-specific value
func TestDatadirEnvironment_DatadirWithEnvironmentOptionGetsEnvironmentSpecificValue(t *testing.T) {
	client, err := quonfig.NewClient(quonfig.WithDataDir(dataDir), quonfig.WithEnvironment("Production"))
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetStringValue("james.test.key", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "test4", val)
}

// datadir with QUONFIG_ENVIRONMENT env var gets environment-specific value
func TestDatadirEnvironment_DatadirWithQUONFIGENVIRONMENTEnvVarGetsEnvironmentSpecificValue(t *testing.T) {
	t.Setenv("QUONFIG_ENVIRONMENT", "Production")
	client, err := quonfig.NewClient(quonfig.WithDataDir(dataDir))
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetStringValue("james.test.key", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "test4", val)
}

// environment option supersedes QUONFIG_ENVIRONMENT env var
func TestDatadirEnvironment_EnvironmentOptionSupersedesQUONFIGENVIRONMENTEnvVar(t *testing.T) {
	t.Setenv("QUONFIG_ENVIRONMENT", "nonexistent")
	client, err := quonfig.NewClient(quonfig.WithDataDir(dataDir), quonfig.WithEnvironment("Production"))
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetStringValue("james.test.key", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "test4", val)
}

// config without environment override returns default value
func TestDatadirEnvironment_ConfigWithoutEnvironmentOverrideReturnsDefaultValue(t *testing.T) {
	client, err := quonfig.NewClient(quonfig.WithDataDir(dataDir), quonfig.WithEnvironment("Production"))
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetStringValue("config.with.only.default.env.row", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "hello from no env row", val)
}

// datadir without environment fails to init
func TestDatadirEnvironment_DatadirWithoutEnvironmentFailsToInit(t *testing.T) {
	_, err := quonfig.NewClient(quonfig.WithDataDir(dataDir))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
}

// datadir with invalid environment fails to init
func TestDatadirEnvironment_DatadirWithInvalidEnvironmentFailsToInit(t *testing.T) {
	_, err := quonfig.NewClient(quonfig.WithDataDir(dataDir), quonfig.WithEnvironment("nonexistent"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}
