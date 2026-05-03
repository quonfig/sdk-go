package quonfig

import (
	"testing"
)

// benchClient builds a Client wired with a runtimeStore + runtimeEvaluator
// (the real eval path, not memStore) so that the benchmark numbers reflect
// what production code actually pays.
func benchClient(b *testing.B) *Client {
	b.Helper()
	envelope := &ConfigEnvelope{
		Meta: Meta{Version: "v1", Environment: "Production"},
		Configs: []ConfigResponse{
			{
				Key: "app.name", ValueType: ValueTypeString,
				Default: RuleSet{Rules: []Rule{
					{Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
						Value: Value{Type: ValueTypeString, Value: "myapp"}},
				}},
			},
			{
				Key: "max.retries", ValueType: ValueTypeInt,
				Default: RuleSet{Rules: []Rule{
					{Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
						Value: Value{Type: ValueTypeInt, Value: int64(5)}},
				}},
			},
			{
				Key: "feature.dark-mode", ValueType: ValueTypeBool, Type: ConfigTypeFeatureFlag,
				Default: RuleSet{Rules: []Rule{
					{Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
						Value: Value{Type: ValueTypeBool, Value: true}},
				}},
			},
			{
				// Realistic: one targeting rule + one default fallback. Hot path
				// has to evaluate criteria, not just return the first rule.
				Key: "feature.beta", ValueType: ValueTypeBool, Type: ConfigTypeFeatureFlag,
				Default: RuleSet{Rules: []Rule{
					{
						Criteria: []Criterion{{
							Operator:     "PROP_IS_ONE_OF",
							PropertyName: "user.id",
							ValueToMatch: &Value{Type: ValueTypeStringList, Value: []string{"u1", "u2", "u3"}},
						}},
						Value: Value{Type: ValueTypeBool, Value: true},
					},
					{Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
						Value: Value{Type: ValueTypeBool, Value: false}},
				}},
			},
		},
	}

	client, err := NewClient()
	if err != nil {
		b.Fatal(err)
	}
	client.installEnvelope(envelope)
	return client
}

func BenchmarkGetStringValue(b *testing.B) {
	c := benchClient(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = c.GetStringValue("app.name", nil)
	}
}

func BenchmarkGetIntValue(b *testing.B) {
	c := benchClient(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = c.GetIntValue("max.retries", nil)
	}
}

func BenchmarkGetBoolValue(b *testing.B) {
	c := benchClient(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = c.GetBoolValue("feature.dark-mode", nil)
	}
}

func BenchmarkFeatureIsOn(b *testing.B) {
	c := benchClient(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.FeatureIsOn("feature.dark-mode", nil)
	}
}

// BenchmarkEvaluateKeyWithTargeting drives the path with criteria evaluation
// against a context — the real cost in production where most flags target.
func BenchmarkEvaluateKeyWithTargeting(b *testing.B) {
	c := benchClient(b)
	ctx := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"id": "u2"})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = c.EvaluateKey("feature.beta", ctx)
	}
}

// BenchmarkEvaluateKeyMiss measures the not-found path (defaultValue branch
// in OpenFeature-style consumers). Most lookups in a misconfigured client
// take this path, so make sure it stays cheap.
func BenchmarkEvaluateKeyMiss(b *testing.B) {
	c := benchClient(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = c.EvaluateKey("does.not.exist", nil)
	}
}
