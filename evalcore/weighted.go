package evalcore

import (
	"fmt"
)

// WeightedValueResolver resolves weighted value distributions to a single value.
//
// It is safe for concurrent use: resolution is a pure function of the
// weighted values, the config key, and the context.
type WeightedValueResolver struct{}

// NewWeightedValueResolver creates a new resolver. The seed is ignored: since
// qfg-9dxb.8 resolution never uses a random source. The parameter is kept so
// the exported signature is unchanged.
func NewWeightedValueResolver(seed int64) *WeightedValueResolver {
	return &WeightedValueResolver{}
}

// Resolve picks a value from the weighted distribution.
//
// If hashByPropertyName is set and the context has a value for that property,
// the selection is deterministic via Murmur3 hash. Otherwise (no hash property
// configured, or the property is absent from the context) it serves the first
// variant (qfg-9dxb.8).
//
// Returns the selected value and its index.
func (w *WeightedValueResolver) Resolve(wv *WeightedValuesData, configKey string, ctx ContextValueGetter) (*Value, int) {
	v, i, _ := w.resolve(wv, configKey, ctx)
	return v, i
}

// resolve is Resolve plus hashPropertyMissing: true when hashByPropertyName is
// set but absent from the context, so the first variant was served.
func (w *WeightedValueResolver) resolve(wv *WeightedValuesData, configKey string, ctx ContextValueGetter) (*Value, int, bool) {
	fraction, hashPropertyMissing := getUserFraction(wv, configKey, ctx)

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
// in the distribution: the Murmur3 hash of the config key and the
// hashByPropertyName value when that property is present in the context, and
// 0.0 (the first variant) otherwise. A property that is present with an empty
// or nil value is hashed like any other value. The bool is true when
// hashByPropertyName is set but absent from the context.
func getUserFraction(wv *WeightedValuesData, configKey string, ctx ContextValueGetter) (float64, bool) {
	if wv.HashByPropertyName == "" {
		return 0, false
	}
	if ctx != nil {
		value, exists := ctx.GetContextValue(wv.HashByPropertyName)
		if exists {
			valueToHash := fmt.Sprintf("%s%v", configKey, value)
			hash, ok := HashZeroToOne(valueToHash)
			if ok {
				return hash, false
			}
			return 0, false
		}
	}
	return 0, true
}
