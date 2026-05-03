package quonfig

import (
	"encoding/json"
	"testing"
)

// FuzzConfigEnvelopeUnmarshal exercises the polymorphic Value.UnmarshalJSON
// path which dispatches on a "type" field. Malformed envelopes flow in via
// both the HTTP poller and the SSE stream — a panic in this code path takes
// down the SDK for any customer hitting the bad payload.
func FuzzConfigEnvelopeUnmarshal(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{}`),
		[]byte(`{"meta":{"version":"v1","environment":"Production"},"configs":[]}`),
		[]byte(`{"configs":[{"key":"k","valueType":"string","default":{"rules":[{"value":{"type":"string","value":"v"}}]}}]}`),
		[]byte(`{"configs":[{"key":"k","valueType":"int","default":{"rules":[{"value":{"type":"int","value":"42"}}]}}]}`),
		[]byte(`{"configs":[{"key":"k","valueType":"bool","default":{"rules":[{"value":{"type":"bool","value":true}}]}}]}`),
		[]byte(`{"configs":[{"key":"k","valueType":"json","default":{"rules":[{"value":{"type":"json","value":{"a":1}}}]}}]}`),
		[]byte(`{"configs":[{"key":"k","valueType":"string_list","default":{"rules":[{"value":{"type":"string_list","value":["a","b"]}}]}}]}`),
		[]byte(`{"configs":[{"key":"k","valueType":"weighted_values","default":{"rules":[{"value":{"type":"weighted_values","value":{"weightedValues":[{"weight":50,"value":{"type":"string","value":"a"}}]}}}]}}]}`),
		[]byte(`{"configs":[{"key":"k","valueType":"provided","default":{"rules":[{"value":{"type":"provided","value":{"source":"ENV_VAR","lookup":"FOO"}}}]}}]}`),
		[]byte(`null`),
		[]byte(``),
		[]byte(`{`),
		[]byte(`{"configs":[null]}`),
		[]byte(`{"configs":[{"key":"k","valueType":"int","default":{"rules":[{"value":{"type":"int","value":"not-a-number"}}]}}]}`),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var env ConfigEnvelope
		_ = json.Unmarshal(data, &env)
	})
}
