// Code generated from integration-test-data/tests/eval/datadir_value_type.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"encoding/json"
	"testing"

	quonfig "github.com/quonfig/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// datadir int config value is loaded as a number, not a string
func TestDatadirValueType_DatadirIntConfigValueIsLoadedAsANumberNotAString(t *testing.T) {
	client, err := quonfig.NewClient(quonfig.WithDataDir(dataDir), quonfig.WithEnvironment("Production"))
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetIntValue("brand.new.int", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, int64(123), val)

	raw, _, found, err := client.EvaluateKey("brand.new.int", nil)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, raw)
	switch raw.Value.(type) {
	case float64, float32, int, int64, int32, json.Number:
		// ok — datadir loader coerced int/double to a real number
	default:
		t.Fatalf("datadir loader must coerce %s to a number, got %T (%v)", "brand.new.int", raw.Value, raw.Value)
	}
}

// datadir double config value is loaded as a number, not a string
func TestDatadirValueType_DatadirDoubleConfigValueIsLoadedAsANumberNotAString(t *testing.T) {
	client, err := quonfig.NewClient(quonfig.WithDataDir(dataDir), quonfig.WithEnvironment("Production"))
	require.NoError(t, err)
	defer client.Close()

	val, ok, err := client.GetFloatValue("my-double-key", nil)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 9.95, val)

	raw, _, found, err := client.EvaluateKey("my-double-key", nil)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, raw)
	switch raw.Value.(type) {
	case float64, float32, int, int64, int32, json.Number:
		// ok — datadir loader coerced int/double to a real number
	default:
		t.Fatalf("datadir loader must coerce %s to a number, got %T (%v)", "my-double-key", raw.Value, raw.Value)
	}
}
