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
// If hashByPropertyName is set, the selection is deterministic via Murmur3
// hash of the config key and the property's value; a missing or nil value is
// hashed as "" (qfg-9dxb.8). If hashByPropertyName is not set, it uses the
// seeded random source on every call.
//
// Returns the selected value and its index.
func (w *WeightedValueResolver) Resolve(wv *WeightedValuesData, configKey string, ctx ContextValueGetter) (*Value, int) {
	v, i, _ := w.resolve(wv, configKey, ctx)
	return v, i
}

// resolve is Resolve plus hashPropertyMissing: true when hashByPropertyName is
// set but its value is missing from the context (or nil).
func (w *WeightedValueResolver) resolve(wv *WeightedValuesData, configKey string, ctx ContextValueGetter) (*Value, int, bool) {
	fraction, hashPropertyMissing := w.getUserFraction(wv, configKey, ctx)

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
			return &v, i, hashPropertyMissing
		}
	}

	// Fallback: return the first value (should not normally be reached)
	if len(wv.WeightedValues) > 0 {
		v := wv.WeightedValues[0].Value
		return &v, 0, hashPropertyMissing
	}
	return nil, -1, hashPropertyMissing
}

// getUserFraction returns a float64 in [0, 1) representing where the user falls
// in the distribution. Deterministic if hashByPropertyName is set: a missing
// or nil value hashes as "", so all such callers share one bucket per flag
// (qfg-9dxb.8). Random otherwise. The bool reports that missing case.
func (w *WeightedValueResolver) getUserFraction(wv *WeightedValuesData, configKey string, ctx ContextValueGetter) (float64, bool) {
	if wv.HashByPropertyName == "" {
		return w.randomFraction(), false
	}
	var value interface{}
	var exists bool
	if ctx != nil {
		value, exists = ctx.GetContextValue(wv.HashByPropertyName)
	}
	missing := !exists || value == nil
	if missing {
		value = ""
	}
	valueToHash := fmt.Sprintf("%s%v", configKey, value)
	if hash, ok := HashZeroToOne(valueToHash); ok {
		return hash, missing
	}
	return w.randomFraction(), missing
}

// randomFraction returns the next value in [0, 1) from the seeded source,
// serialized so concurrent callers cannot corrupt its state.
func (w *WeightedValueResolver) randomFraction() float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rng.Float64()
}
