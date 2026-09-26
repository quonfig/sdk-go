package evalcore

import (
	"fmt"
	"math/rand"
	"sync"
)

// WeightedValueResolver resolves weighted value distributions to a single value.
//
// It is safe for concurrent use. A single resolver is shared by every goroutine
// evaluating through an Evaluator, and *rand.Rand is not goroutine-safe, so the
// random fallback source is guarded by mu (qfg-9dxb.1). A mutex, rather than
// the math/rand/v2 top-level functions, keeps a fixed seed reproducible for
// NewEvaluatorWithSeed.
type WeightedValueResolver struct {
	mu  sync.Mutex
	rng *rand.Rand
}

// NewWeightedValueResolver creates a new resolver with a seeded random source.
// The returned resolver is safe for concurrent use.
func NewWeightedValueResolver(seed int64) *WeightedValueResolver {
	src := rand.NewSource(seed)
	return &WeightedValueResolver{
		rng: rand.New(src),
	}
}

// Resolve picks a value from the weighted distribution.
//
// If hashByPropertyName is set and the context has a value for that property,
// the selection is deterministic via Murmur3 hash. Otherwise, it falls back to
// the seeded random source.
//
// Returns the selected value and its index.
func (w *WeightedValueResolver) Resolve(wv *WeightedValuesData, configKey string, ctx ContextValueGetter) (*Value, int) {
	fraction := w.getUserFraction(wv, configKey, ctx)

	totalWeight := 0
	for _, entry := range wv.WeightedValues {
		totalWeight += entry.Weight
	}

	threshold := fraction * float64(totalWeight)

	runningSum := 0
	for i, entry := range wv.WeightedValues {
		runningSum += entry.Weight
		if float64(runningSum) >= threshold {
			v := entry.Value // copy
			return &v, i
		}
	}

	// Fallback: return the first value (should not normally be reached)
	if len(wv.WeightedValues) > 0 {
		v := wv.WeightedValues[0].Value
		return &v, 0
	}
	return nil, -1
}

// getUserFraction returns a float64 in [0, 1) representing where the user falls
// in the distribution. Deterministic if hashByPropertyName is set and present
// in context; random otherwise.
func (w *WeightedValueResolver) getUserFraction(wv *WeightedValuesData, configKey string, ctx ContextValueGetter) float64 {
	if wv.HashByPropertyName != "" && ctx != nil {
		value, exists := ctx.GetContextValue(wv.HashByPropertyName)
		if exists {
			valueToHash := fmt.Sprintf("%s%v", configKey, value)
			hash, ok := HashZeroToOne(valueToHash)
			if ok {
				return hash
			}
		}
	}

	return w.randomFraction()
}

// randomFraction returns the next value in [0, 1) from the seeded source,
// serialized so concurrent callers cannot corrupt its state.
func (w *WeightedValueResolver) randomFraction() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rng.Float64()
}
