package evalcore

import (
	"time"
)

// ConfigStoreGetter retrieves configs by key.
type ConfigStoreGetter interface {
	GetConfig(key string) (*Config, bool)
}

// EvalMatch is the result of evaluating a config against a context.
type EvalMatch struct {
	IsMatch   bool
	Value     *Value
	RuleIndex int
	// IsWeighted is true when the matched value was a weighted_values block that
	// was resolved through the WeightedValueResolver. It is the unambiguous
	// signal for the SPLIT reason: WeightedValueIndex alone cannot distinguish a
	// weighted value landing in bucket 0 from a non-weighted match.
	IsWeighted bool
	// WeightedValueIndex is the 0-based bucket index when IsWeighted is true; 0
	// otherwise. Only meaningful when IsWeighted is true.
	WeightedValueIndex int
	// MissingHashProperty is the weighted value's hashByPropertyName when that
	// property was absent from the context, so the rollout served its first
	// variant (qfg-9dxb.8). Empty otherwise.
	MissingHashProperty string
}

// Evaluator is the main evaluation engine. It evaluates configs against contexts,
// resolving rules, operators, segments, and weighted values.
type Evaluator struct {
	configStore ConfigStoreGetter
	weighted    *WeightedValueResolver
}

// NewEvaluator creates a new Evaluator.
func NewEvaluator(configStore ConfigStoreGetter) *Evaluator {
	return &Evaluator{
		configStore: configStore,
		weighted:    NewWeightedValueResolver(time.Now().UnixNano()),
	}
}

// NewEvaluatorWithSeed creates a new Evaluator. The seed is ignored since
// weighted resolution stopped using a random source (qfg-9dxb.8); it is kept so
// the exported signature is unchanged.
func NewEvaluatorWithSeed(configStore ConfigStoreGetter, seed int64) *Evaluator {
	return &Evaluator{
		configStore: configStore,
		weighted:    NewWeightedValueResolver(seed),
	}
}

// EvaluateConfig evaluates a config for the given environment and context.
//
// Evaluation flow:
//  1. Find the environment block matching envID (if any)
//  2. Iterate its rules top-to-bottom; first match wins
//  3. If no env-specific match, fall back to default.rules
//  4. For each rule, all criteria must match (AND logic)
//  5. If matched value is weighted_values, resolve through WeightedValueResolver
func (e *Evaluator) EvaluateConfig(cfg *Config, envID string, ctx ContextValueGetter) *EvalMatch {
	return e.evaluateConfig(cfg, envID, ctx, nil)
}

// evaluateConfig is EvaluateConfig plus segPath: the keys of the configs that
// are currently being evaluated above this one through IN_SEG / NOT_IN_SEG.
// It lets segment resolution detect a reference cycle (a segment that is, via
// any chain, IN_SEG itself) and stop instead of recursing until the runtime
// kills the process with an unrecoverable stack overflow (qfg-9dxb.4). It is a
// path, not a global visited set, so a diamond (two segments that both
// reference a third) still resolves. It stays nil, and costs nothing, for
// configs that never reference a segment.
func (e *Evaluator) evaluateConfig(cfg *Config, envID string, ctx ContextValueGetter, segPath []string) *EvalMatch {
	if ctx == nil {
		ctx = EmptyContext{}
	}

	// Try environment-specific rules first
	if envID != "" {
		env := cfg.FindEnvironment(envID)
		if env != nil {
			if match := e.evaluateRules(cfg, env.Rules, ctx, 0, segPath); match != nil {
				return match
			}
		}
	}

	// Fall back to default rules
	if match := e.evaluateRules(cfg, cfg.Default.Rules, ctx, 0, segPath); match != nil {
		return match
	}

	return &EvalMatch{IsMatch: false}
}

// evaluateRules tries rules in order, returning the first match.
func (e *Evaluator) evaluateRules(cfg *Config, rules []Rule, ctx ContextValueGetter, ruleIndexOffset int, segPath []string) *EvalMatch {
	for i, rule := range rules {
		if e.evaluateAllCriteria(cfg, rule.Criteria, ctx, segPath) {
			value := rule.Value // copy
			match := &EvalMatch{
				IsMatch:   true,
				Value:     &value,
				RuleIndex: ruleIndexOffset + i,
			}

			// Resolve weighted values
			if value.Type == ValueTypeWeightedValues {
				wvData := value.WeightedValuesValue()
				if wvData != nil {
					resolved, wvIndex, hashPropertyMissing := e.weighted.resolve(wvData, cfg.Key, ctx)
					if resolved != nil {
						match.Value = resolved
						match.IsWeighted = true
						match.WeightedValueIndex = wvIndex
						if hashPropertyMissing {
							match.MissingHashProperty = wvData.HashByPropertyName
						}
					}
				}
			}

			return match
		}
	}
	return nil
}

// evaluateAllCriteria returns true if ALL criteria match (AND logic).
func (e *Evaluator) evaluateAllCriteria(cfg *Config, criteria []Criterion, ctx ContextValueGetter, segPath []string) bool {
	for _, criterion := range criteria {
		if !e.evaluateSingleCriterion(cfg, criterion, ctx, segPath) {
			return false
		}
	}
	return true
}

// evaluateSingleCriterion evaluates one criterion, handling special properties
// and segment resolution.
func (e *Evaluator) evaluateSingleCriterion(cfg *Config, criterion Criterion, ctx ContextValueGetter, segPath []string) bool {
	contextValue, contextExists := ctx.GetContextValue(criterion.PropertyName)

	// Handle magic current-time properties at the criterion level
	if criterion.PropertyName == "prefab.current-time" ||
		criterion.PropertyName == "quonfig.current-time" ||
		criterion.PropertyName == "reforge.current-time" {
		contextValue = time.Now().UTC().UnixMilli()
		contextExists = true
	}

	// Build a segment resolver that recursively evaluates segment configs
	segmentResolver := func(segmentKey string) (bool, bool) {
		if e.configStore == nil {
			return false, false
		}
		// A reference back to any config on the current evaluation path is a
		// cycle. Treat it like a missing segment (IN_SEG false, NOT_IN_SEG
		// true) rather than recursing forever.
		if segmentKey == cfg.Key || containsKey(segPath, segmentKey) {
			return false, false
		}
		segConfig, exists := e.configStore.GetConfig(segmentKey)
		if !exists {
			return false, false
		}
		// Full-slice expression: force a copy on append so sibling criteria
		// never share (and overwrite) one backing array.
		childPath := append(segPath[:len(segPath):len(segPath)], cfg.Key)
		// Evaluate the segment config (segments have no environment, use default rules)
		segMatch := e.evaluateConfig(segConfig, "", ctx, childPath)
		if !segMatch.IsMatch || segMatch.Value == nil {
			return false, false
		}
		return segMatch.Value.BoolValue(), true
	}

	return EvaluateCriterion(contextValue, contextExists, criterion, segmentResolver)
}

func containsKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}
