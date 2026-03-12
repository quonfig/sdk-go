package evalcore

import (
	"testing"
)

func TestWeightedValueResolver_Deterministic(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "control"}},
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "treatment"}},
		},
		HashByPropertyName: "user.key",
	}

	resolver := NewWeightedValueResolver(42)

	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"key": "user-abc",
	}))

	val1, idx1 := resolver.Resolve(wv, "my-flag", ctx)
	val2, idx2 := resolver.Resolve(wv, "my-flag", ctx)

	if val1 == nil || val2 == nil {
		t.Fatal("expected non-nil values")
	}
	if val1.StringValue() != val2.StringValue() {
		t.Errorf("expected same values, got %q and %q", val1.StringValue(), val2.StringValue())
	}
	if idx1 != idx2 {
		t.Errorf("expected same indices, got %d and %d", idx1, idx2)
	}
}

func TestWeightedValueResolver_DifferentKeys(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "A"}},
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "B"}},
		},
		HashByPropertyName: "user.key",
	}

	resolver := NewWeightedValueResolver(42)

	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
			"key": i,
		}))
		val, _ := resolver.Resolve(wv, "test-flag", ctx)
		if val == nil {
			t.Fatalf("expected non-nil value for key %d", i)
		}
		seen[val.StringValue()] = true
	}

	if !seen["A"] {
		t.Error("should see variant A")
	}
	if !seen["B"] {
		t.Error("should see variant B")
	}
}

func TestWeightedValueResolver_UnevenWeights(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 90, Value: Value{Type: ValueTypeString, Value: "heavy"}},
			{Weight: 10, Value: Value{Type: ValueTypeString, Value: "light"}},
		},
		HashByPropertyName: "user.key",
	}

	resolver := NewWeightedValueResolver(42)

	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
			"key": i,
		}))
		val, _ := resolver.Resolve(wv, "test-flag", ctx)
		if val == nil {
			t.Fatalf("expected non-nil value for key %d", i)
		}
		counts[val.StringValue()]++
	}

	if counts["heavy"] < 800 {
		t.Errorf("heavy should dominate, got %d", counts["heavy"])
	}
	if counts["light"] > 200 {
		t.Errorf("light should be minority, got %d", counts["light"])
	}
}

func TestWeightedValueResolver_NoHashProperty_UsesRandom(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "A"}},
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "B"}},
		},
	}

	resolver := NewWeightedValueResolver(42)
	val1, idx1 := resolver.Resolve(wv, "my-flag", nil)
	if val1 == nil {
		t.Fatal("expected non-nil value")
	}

	resolver2 := NewWeightedValueResolver(42)
	val2, idx2 := resolver2.Resolve(wv, "my-flag", nil)
	if val2 == nil {
		t.Fatal("expected non-nil value")
	}

	if val1.StringValue() != val2.StringValue() {
		t.Errorf("expected same values with same seed, got %q and %q", val1.StringValue(), val2.StringValue())
	}
	if idx1 != idx2 {
		t.Errorf("expected same indices with same seed, got %d and %d", idx1, idx2)
	}
}

func TestWeightedValueResolver_SingleValue(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 100, Value: Value{Type: ValueTypeString, Value: "only"}},
		},
		HashByPropertyName: "user.key",
	}

	resolver := NewWeightedValueResolver(42)
	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"key": "anyone",
	}))

	val, idx := resolver.Resolve(wv, "test", ctx)
	if val == nil {
		t.Fatal("expected non-nil value")
	}
	if val.StringValue() != "only" {
		t.Errorf("expected 'only', got %q", val.StringValue())
	}
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}
}

func TestWeightedValueResolver_HashPropertyNotInContext(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "A"}},
			{Weight: 50, Value: Value{Type: ValueTypeString, Value: "B"}},
		},
		HashByPropertyName: "user.key",
	}

	resolver := NewWeightedValueResolver(42)
	ctx := NewContextSet().WithNamedContext(NewNamedContext("user", map[string]interface{}{
		"email": "test@example.com",
	}))

	val, _ := resolver.Resolve(wv, "test", ctx)
	if val == nil {
		t.Fatal("expected non-nil value")
	}
	sv := val.StringValue()
	if sv != "A" && sv != "B" {
		t.Errorf("expected A or B, got %q", sv)
	}
}

func TestHashZeroToOne_Determinism(t *testing.T) {
	val1, ok1 := HashZeroToOne("test-key-123")
	val2, ok2 := HashZeroToOne("test-key-123")

	if !ok1 || !ok2 {
		t.Fatal("expected ok to be true")
	}
	if val1 != val2 {
		t.Errorf("expected same values, got %f and %f", val1, val2)
	}
	if val1 < 0.0 || val1 >= 1.0 {
		t.Errorf("expected value in [0, 1), got %f", val1)
	}
}

func TestHashZeroToOne_Distribution(t *testing.T) {
	buckets := make([]int, 10)
	for i := 0; i < 10000; i++ {
		val, ok := HashZeroToOne(ToString(i))
		if !ok {
			t.Fatalf("expected ok for key %d", i)
		}
		if val < 0.0 || val >= 1.0 {
			t.Fatalf("value out of range: %f", val)
		}
		bucket := int(val * 10)
		if bucket >= 10 {
			bucket = 9
		}
		buckets[bucket]++
	}

	for i, count := range buckets {
		if count < 700 {
			t.Errorf("bucket %d is too small: %d", i, count)
		}
		if count > 1300 {
			t.Errorf("bucket %d is too large: %d", i, count)
		}
	}
}
