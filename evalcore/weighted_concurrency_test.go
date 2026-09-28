package evalcore

import (
	"sync"
	"testing"
)

// TestWeightedValueResolver_ConcurrentRandomFallback guards qfg-9dxb.1 (audit
// C1). One WeightedValueResolver is shared by every goroutine evaluating
// through an Evaluator. When hashByPropertyName was missing from the context
// the resolver used to fall back to a shared *rand.Rand, which raced and could
// panic. Since qfg-9dxb.8 that path serves the first variant with no random
// source; this test keeps it race-free under -race.
func TestWeightedValueResolver_ConcurrentRandomFallback(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 1, Value: Value{Type: ValueTypeString, Value: "A"}},
			{Weight: 1, Value: Value{Type: ValueTypeString, Value: "B"}},
		},
		// Hash key is set but absent from the (empty) context, so every
		// Resolve takes the missing-property fallback path.
		HashByPropertyName: "user.key",
	}
	resolver := NewWeightedValueResolver(42)

	const goroutines = 16
	const iterations = 20000
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				val, idx := resolver.Resolve(wv, "rollout", EmptyContext{})
				if val == nil || idx < 0 || idx > 1 {
					t.Errorf("unexpected resolve result: val=%v idx=%d", val, idx)
					return
				}
			}
		}()
	}
	wg.Wait()
}
