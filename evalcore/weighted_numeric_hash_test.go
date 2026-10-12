package evalcore

import (
	"testing"
)

// A numeric hashByPropertyName value hashes as its plain decimal digits, the
// way sdk-node (String(n)) and sdk-ruby (to_s) render it (qfg-goi1.9, option
// (a) approved 2026-10-12). Before the fix a float64 >= 1e6 rendered with %v as
// 1.234567e+06, so a frontend user whose numeric id reached api-delivery as a
// JSON float64 bucketed differently than the same user in sdk-node.
func TestWeightedValueResolver_NumericHashInputIsPlainDigits(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "A"}},
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "B"}},
		},
		HashByPropertyName: "user.id",
	}
	cases := []struct {
		name  string
		value interface{}
		want  string
	}{
		{"float64 >= 1e6", float64(1234567), "1234567"},
		{"float64 prod max", float64(2515151), "2515151"},
		{"float64 1e6", float64(1e6), "1000000"},
		{"float64 1e15", float64(1e15), "1000000000000000"},
		{"float64 negative", float64(-2000000), "-2000000"},
		{"float64 negative zero", negZero(), "0"},
		{"float32 >= 1e6", float32(2097152), "2097152"},
		{"int", 1234567, "1234567"},
		{"int64", int64(1234567), "1234567"},
		{"uint64", uint64(1234567), "1234567"},
		{"float64 small whole", float64(42), "42"},
		{"float64 fractional unchanged", 1.5, "1.5"},
		{"string unchanged", "1234567", "1234567"},
		{"bool unchanged", true, "true"},
	}
	resolver := NewWeightedValueResolver(1)
	for _, tc := range cases {
		ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{"id": tc.value}))
		got, _ := resolver.getUserFraction(wv, "my-flag", ctx)
		want, ok := HashZeroToOne("my-flag" + tc.want)
		if !ok {
			t.Fatalf("%s: HashZeroToOne failed", tc.name)
		}
		if got != want {
			t.Errorf("%s: fraction for %#v = %v, want hash of %q (%v)", tc.name, tc.value, got, "my-flag"+tc.want, want)
		}
	}
}

func negZero() float64 {
	z := 0.0
	return -z
}
