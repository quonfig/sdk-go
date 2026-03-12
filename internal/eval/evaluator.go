package eval

import (
	"time"

	quonfig "github.com/quonfig/sdk-go"
	sharedeval "github.com/quonfig/eval"
)

// FullConfig is a config with all environments, matching the raw JSON format.
// This is used for evaluation where we need to look up configs by environment.
type FullConfig struct {
	ID              string                `json:"id"`
	Key             string                `json:"key"`
	Type            quonfig.ConfigType    `json:"type"`
	ValueType       quonfig.ValueType     `json:"valueType"`
	SendToClientSDK bool                  `json:"sendToClientSdk"`
	Default         quonfig.RuleSet       `json:"default"`
	Environments    []quonfig.Environment `json:"environments,omitempty"`
}

// FindEnvironment returns the environment block matching the given environment ID, or nil.
func (c *FullConfig) FindEnvironment(envID string) *quonfig.Environment {
	for i := range c.Environments {
		if c.Environments[i].ID == envID {
			return &c.Environments[i]
		}
	}
	return nil
}

// ConfigStoreGetter retrieves configs by key.
type ConfigStoreGetter interface {
	GetConfig(key string) (*FullConfig, bool)
}

// EvalMatch is the result of evaluating a config against a context.
type EvalMatch struct {
	IsMatch            bool
	Value              *quonfig.Value
	RuleIndex          int
	WeightedValueIndex int
}

// ContextValueGetter is re-exported from the shared eval package.
type ContextValueGetter = sharedeval.ContextValueGetter

// EmptyContext is re-exported from the shared eval package.
type EmptyContext = sharedeval.EmptyContext

// Evaluator is the main evaluation engine. It delegates to the shared eval package.
type Evaluator struct {
	sharedEval *sharedeval.Evaluator
	configStore ConfigStoreGetter
}

// sharedConfigStoreAdapter adapts our ConfigStoreGetter to the shared package's interface.
type sharedConfigStoreAdapter struct {
	store ConfigStoreGetter
}

func (a *sharedConfigStoreAdapter) GetConfig(key string) (*sharedeval.Config, bool) {
	cfg, ok := a.store.GetConfig(key)
	if !ok {
		return nil, false
	}
	return fullConfigToShared(cfg), true
}

// NewEvaluator creates a new Evaluator.
func NewEvaluator(configStore ConfigStoreGetter) *Evaluator {
	adapter := &sharedConfigStoreAdapter{store: configStore}
	return &Evaluator{
		sharedEval:  sharedeval.NewEvaluator(adapter),
		configStore: configStore,
	}
}

// NewEvaluatorWithSeed creates a new Evaluator with a fixed random seed (for testing).
func NewEvaluatorWithSeed(configStore ConfigStoreGetter, seed int64) *Evaluator {
	adapter := &sharedConfigStoreAdapter{store: configStore}
	return &Evaluator{
		sharedEval:  sharedeval.NewEvaluatorWithSeed(adapter, seed),
		configStore: configStore,
	}
}

// NewEvaluatorWithTimeSeed creates a new Evaluator with a time-based seed.
func NewEvaluatorWithTimeSeed(configStore ConfigStoreGetter) *Evaluator {
	return NewEvaluatorWithSeed(configStore, time.Now().UnixNano())
}

// EvaluateConfig evaluates a config for the given environment and context.
func (e *Evaluator) EvaluateConfig(cfg *FullConfig, envID string, ctx ContextValueGetter) *EvalMatch {
	sharedCfg := fullConfigToShared(cfg)
	sharedMatch := e.sharedEval.EvaluateConfig(sharedCfg, envID, ctx)
	return sharedMatchToLocal(sharedMatch)
}

// fullConfigToShared converts a FullConfig to the shared eval.Config type.
func fullConfigToShared(fc *FullConfig) *sharedeval.Config {
	cfg := &sharedeval.Config{
		ID:              fc.ID,
		Key:             fc.Key,
		Type:            sharedeval.ConfigType(fc.Type),
		ValueType:       sharedeval.ValueType(fc.ValueType),
		SendToClientSDK: fc.SendToClientSDK,
		Default:         ruleSetToShared(fc.Default),
		Environments:    make([]sharedeval.Environment, len(fc.Environments)),
	}
	for i, env := range fc.Environments {
		cfg.Environments[i] = environmentToShared(env)
	}
	return cfg
}

// ruleSetToShared converts a quonfig.RuleSet to the shared eval type.
func ruleSetToShared(rs quonfig.RuleSet) sharedeval.RuleSet {
	shared := sharedeval.RuleSet{
		Rules: make([]sharedeval.Rule, len(rs.Rules)),
	}
	for i, rule := range rs.Rules {
		shared.Rules[i] = ruleToShared(rule)
	}
	return shared
}

// environmentToShared converts a quonfig.Environment to the shared eval type.
func environmentToShared(env quonfig.Environment) sharedeval.Environment {
	shared := sharedeval.Environment{
		ID:    env.ID,
		Rules: make([]sharedeval.Rule, len(env.Rules)),
	}
	for i, rule := range env.Rules {
		shared.Rules[i] = ruleToShared(rule)
	}
	return shared
}

// ruleToShared converts a quonfig.Rule to the shared eval type.
func ruleToShared(rule quonfig.Rule) sharedeval.Rule {
	shared := sharedeval.Rule{
		Criteria: make([]sharedeval.Criterion, len(rule.Criteria)),
		Value:    valueToShared(rule.Value),
	}
	for i, c := range rule.Criteria {
		shared.Criteria[i] = criterionToShared(c)
	}
	return shared
}

// criterionToShared converts a quonfig.Criterion to the shared eval type.
func criterionToShared(c quonfig.Criterion) sharedeval.Criterion {
	shared := sharedeval.Criterion{
		PropertyName: c.PropertyName,
		Operator:     c.Operator,
	}
	if c.ValueToMatch != nil {
		sv := valueToShared(*c.ValueToMatch)
		shared.ValueToMatch = &sv
	}
	return shared
}

// valueToShared converts a quonfig.Value to the shared eval type.
func valueToShared(v quonfig.Value) sharedeval.Value {
	sv := sharedeval.Value{
		Type:         sharedeval.ValueType(v.Type),
		Confidential: v.Confidential,
		DecryptWith:  v.DecryptWith,
	}

	// Convert value payload - most types are identical
	switch val := v.Value.(type) {
	case *quonfig.WeightedValuesData:
		if val != nil {
			sv.Value = weightedValuesDataToShared(val)
		}
	case *quonfig.ProvidedData:
		if val != nil {
			sv.Value = &sharedeval.ProvidedData{
				Source: val.Source,
				Lookup: val.Lookup,
			}
		}
	case *quonfig.SchemaData:
		if val != nil {
			sv.Value = &sharedeval.SchemaData{
				SchemaType: val.SchemaType,
				Schema:     val.Schema,
			}
		}
	default:
		// bool, int64, float64, string, []string, map[string]interface{} etc. -- same across packages
		sv.Value = v.Value
	}
	return sv
}

// weightedValuesDataToShared converts quonfig.WeightedValuesData to the shared type.
func weightedValuesDataToShared(wv *quonfig.WeightedValuesData) *sharedeval.WeightedValuesData {
	shared := &sharedeval.WeightedValuesData{
		HashByPropertyName: wv.HashByPropertyName,
		WeightedValues:     make([]sharedeval.WeightedValue, len(wv.WeightedValues)),
	}
	for i, entry := range wv.WeightedValues {
		shared.WeightedValues[i] = sharedeval.WeightedValue{
			Weight: entry.Weight,
			Value:  valueToShared(entry.Value),
		}
	}
	return shared
}

// sharedMatchToLocal converts a shared EvalMatch back to the local type.
func sharedMatchToLocal(match *sharedeval.EvalMatch) *EvalMatch {
	if match == nil {
		return &EvalMatch{IsMatch: false}
	}
	result := &EvalMatch{
		IsMatch:            match.IsMatch,
		RuleIndex:          match.RuleIndex,
		WeightedValueIndex: match.WeightedValueIndex,
	}
	if match.Value != nil {
		localVal := sharedValueToLocal(*match.Value)
		result.Value = &localVal
	}
	return result
}

// sharedValueToLocal converts a shared eval.Value back to a quonfig.Value.
func sharedValueToLocal(sv sharedeval.Value) quonfig.Value {
	v := quonfig.Value{
		Type:         quonfig.ValueType(sv.Type),
		Confidential: sv.Confidential,
		DecryptWith:  sv.DecryptWith,
	}

	switch val := sv.Value.(type) {
	case *sharedeval.WeightedValuesData:
		if val != nil {
			v.Value = sharedWeightedValuesDataToLocal(val)
		}
	case *sharedeval.ProvidedData:
		if val != nil {
			v.Value = &quonfig.ProvidedData{
				Source: val.Source,
				Lookup: val.Lookup,
			}
		}
	case *sharedeval.SchemaData:
		if val != nil {
			v.Value = &quonfig.SchemaData{
				SchemaType: val.SchemaType,
				Schema:     val.Schema,
			}
		}
	default:
		v.Value = sv.Value
	}
	return v
}

// sharedWeightedValuesDataToLocal converts shared WeightedValuesData back to quonfig type.
func sharedWeightedValuesDataToLocal(wv *sharedeval.WeightedValuesData) *quonfig.WeightedValuesData {
	local := &quonfig.WeightedValuesData{
		HashByPropertyName: wv.HashByPropertyName,
		WeightedValues:     make([]quonfig.WeightedValue, len(wv.WeightedValues)),
	}
	for i, entry := range wv.WeightedValues {
		local.WeightedValues[i] = quonfig.WeightedValue{
			Weight: entry.Weight,
			Value:  sharedValueToLocal(entry.Value),
		}
	}
	return local
}
