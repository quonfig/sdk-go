package evalcore

import (
	"sync"
	"testing"
)

// TestWeightedValueResolver_ConcurrentRandomFallback guards qfg-9dxb.1 (audit
// C1). One WeightedValueResolver is shared by every goroutine evaluating
// through an Evaluator, and when hashByPropertyName is missing from the
// context the resolver falls back to its random source. That source must be
// goroutine-safe: an unsynchronized *rand.Rand races (caught by -race) and can
// panic with "index out of range [-1]", killing the host process.
func TestWeightedValueResolver_ConcurrentRandomFallback(t *testing.T) {
	wv := &WeightedValuesData{
		WeightedValues: []WeightedValue{
			{Weight: 1, Value: Value{Type: ValueTypeString, Value: "A"}},
			{Weight: 1, Value: Value{Type: ValueTypeString, Value: "B"}},
		},
		// Hash key is set but absent from the (empty) context, so every
		// Resolve takes the random fallback path.
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
