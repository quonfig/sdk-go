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
	IsMatch            bool
	Value              *Value
	RuleIndex          int
	WeightedValueIndex int
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

// NewEvaluatorWithSeed creates a new Evaluator with a fixed random seed (for testing).
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
	if ctx == nil {
		ctx = EmptyContext{}
	}

	// Try environment-specific rules first
	if envID != "" {
		env := cfg.FindEnvironment(envID)
		if env != nil {
			if match := e.evaluateRules(cfg, env.Rules, ctx, 0); match != nil {
				return match
			}
		}
	}

	// Fall back to default rules
	if match := e.evaluateRules(cfg, cfg.Default.Rules, ctx, 0); match != nil {
		return match
	}

	return &EvalMatch{IsMatch: false}
}

// evaluateRules tries rules in order, returning the first match.
func (e *Evaluator) evaluateRules(cfg *Config, rules []Rule, ctx ContextValueGetter, ruleIndexOffset int) *EvalMatch {
	for i, rule := range rules {
		if e.evaluateAllCriteria(cfg, rule.Criteria, ctx) {
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
					resolved, wvIndex := e.weighted.Resolve(wvData, cfg.Key, ctx)
					if resolved != nil {
						match.Value = resolved
						match.WeightedValueIndex = wvIndex
					}
				}
			}

			return match
		}
	}
	return nil
}

// evaluateAllCriteria returns true if ALL criteria match (AND logic).
func (e *Evaluator) evaluateAllCriteria(cfg *Config, criteria []Criterion, ctx ContextValueGetter) bool {
	for _, criterion := range criteria {
		if !e.evaluateSingleCriterion(cfg, criterion, ctx) {
			return false
		}
	}
	return true
}

// evaluateSingleCriterion evaluates one criterion, handling special properties
// and segment resolution.
func (e *Evaluator) evaluateSingleCriterion(cfg *Config, criterion Criterion, ctx ContextValueGetter) bool {
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
		segConfig, exists := e.configStore.GetConfig(segmentKey)
		if !exists {
			return false, false
		}
		// Evaluate the segment config (segments have no environment, use default rules)
		segMatch := e.EvaluateConfig(segConfig, "", ctx)
		if !segMatch.IsMatch || segMatch.Value == nil {
			return false, false
		}
		return segMatch.Value.BoolValue(), true
	}

	return EvaluateCriterion(contextValue, contextExists, criterion, segmentResolver)
}
