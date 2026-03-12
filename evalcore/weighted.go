package evalcore

import (
	"fmt"
	"math/rand"
)

// WeightedValueResolver resolves weighted value distributions to a single value.
type WeightedValueResolver struct {
	rng *rand.Rand
}

// NewWeightedValueResolver creates a new resolver with a seeded random source.
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

	return w.rng.Float64()
}
