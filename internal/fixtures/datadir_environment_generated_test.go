package fixtures

// Code generated from integration-test-data/tests/eval/datadir_environment.yaml. DO NOT EDIT.

import (
	"testing"

	quonfig "github.com/quonfig/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDataDir = "../../../integration-test-data/data/integration-tests"

func TestDatadirEnvironment_DatadirWithEnvironmentOptionGetsEnvironmentSpecificValue(t *testing.T) {
	client, err := quonfig.NewClient(
		quonfig.WithDataDir(testDataDir),
		quonfig.WithEnvironment("Production"),
	)
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetStringValue("james.test.key", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "test4", val)
}

func TestDatadirEnvironment_DatadirWithQuonfigEnvironmentEnvVarGetsEnvironmentSpecificValue(t *testing.T) {
	t.Setenv("QUONFIG_ENVIRONMENT", "Production")

	client, err := quonfig.NewClient(
		quonfig.WithDataDir(testDataDir),
	)
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetStringValue("james.test.key", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "test4", val)
}

func TestDatadirEnvironment_EnvironmentOptionSupersedesQuonfigEnvironmentEnvVar(t *testing.T) {
	t.Setenv("QUONFIG_ENVIRONMENT", "nonexistent")

	client, err := quonfig.NewClient(
		quonfig.WithDataDir(testDataDir),
		quonfig.WithEnvironment("Production"),
	)
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetStringValue("james.test.key", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "test4", val)
}

func TestDatadirEnvironment_ConfigWithoutEnvironmentOverrideReturnsDefaultValue(t *testing.T) {
	client, err := quonfig.NewClient(
		quonfig.WithDataDir(testDataDir),
		quonfig.WithEnvironment("Production"),
	)
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetStringValue("config.with.only.default.env.row", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "hello from no env row", val)
}

func TestDatadirEnvironment_DatadirWithoutEnvironmentFailsToInit(t *testing.T) {
	_, err := quonfig.NewClient(
		quonfig.WithDataDir(testDataDir),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment")
}

func TestDatadirEnvironment_DatadirWithInvalidEnvironmentFailsToInit(t *testing.T) {
	_, err := quonfig.NewClient(
		quonfig.WithDataDir(testDataDir),
		quonfig.WithEnvironment("nonexistent"),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}
