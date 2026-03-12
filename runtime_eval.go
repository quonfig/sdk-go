package quonfig

import sharedeval "github.com/quonfig/eval"

type runtimeEvaluator struct {
	shared *sharedeval.Evaluator
}

type sharedConfigStoreAdapter struct {
	store configStore
}

func newRuntimeEvaluator(store configStore) *runtimeEvaluator {
	return &runtimeEvaluator{
		shared: sharedeval.NewEvaluator(&sharedConfigStoreAdapter{store: store}),
	}
}

func (a *sharedConfigStoreAdapter) GetConfig(key string) (*sharedeval.Config, bool) {
	cfg, ok := a.store.Get(key)
	if !ok {
		return nil, false
	}
	return configResponseToShared(cfg), true
}

func (e *runtimeEvaluator) EvaluateConfigResponse(cfg *ConfigResponse, envID string, ctx *ContextSet) *Value {
	var getter sharedeval.ContextValueGetter = sharedeval.EmptyContext{}
	if ctx != nil {
		getter = ctx
	}

	match := e.shared.EvaluateConfig(configResponseToShared(cfg), envID, getter)
	if match == nil || !match.IsMatch || match.Value == nil {
		return nil
	}

	value := sharedValueToLocal(*match.Value)
	return &value
}

func configResponseToShared(cfg *ConfigResponse) *sharedeval.Config {
	sharedCfg := &sharedeval.Config{
		ID:              cfg.ID,
		Key:             cfg.Key,
		Type:            sharedeval.ConfigType(cfg.Type),
		ValueType:       sharedeval.ValueType(cfg.ValueType),
		SendToClientSDK: cfg.SendToClientSDK,
		Default:         ruleSetToShared(cfg.Default),
	}
	if cfg.Environment != nil {
		sharedCfg.Environments = []sharedeval.Environment{environmentToShared(*cfg.Environment)}
	}
	return sharedCfg
}

func ruleSetToShared(rs RuleSet) sharedeval.RuleSet {
	shared := sharedeval.RuleSet{
		Rules: make([]sharedeval.Rule, len(rs.Rules)),
	}
	for i, rule := range rs.Rules {
		shared.Rules[i] = ruleToShared(rule)
	}
	return shared
}

func environmentToShared(env Environment) sharedeval.Environment {
	shared := sharedeval.Environment{
		ID:    env.ID,
		Rules: make([]sharedeval.Rule, len(env.Rules)),
	}
	for i, rule := range env.Rules {
		shared.Rules[i] = ruleToShared(rule)
	}
	return shared
}

func ruleToShared(rule Rule) sharedeval.Rule {
	shared := sharedeval.Rule{
		Criteria: make([]sharedeval.Criterion, len(rule.Criteria)),
		Value:    valueToShared(rule.Value),
	}
	for i, criterion := range rule.Criteria {
		shared.Criteria[i] = criterionToShared(criterion)
	}
	return shared
}

func criterionToShared(c Criterion) sharedeval.Criterion {
	shared := sharedeval.Criterion{
		PropertyName: c.PropertyName,
		Operator:     c.Operator,
	}
	if c.ValueToMatch != nil {
		value := valueToShared(*c.ValueToMatch)
		shared.ValueToMatch = &value
	}
	return shared
}

func valueToShared(v Value) sharedeval.Value {
	shared := sharedeval.Value{
		Type:         sharedeval.ValueType(v.Type),
		Confidential: v.Confidential,
		DecryptWith:  v.DecryptWith,
	}

	switch value := v.Value.(type) {
	case *WeightedValuesData:
		if value != nil {
			shared.Value = weightedValuesDataToShared(value)
		}
	case *ProvidedData:
		if value != nil {
			shared.Value = &sharedeval.ProvidedData{
				Source: value.Source,
				Lookup: value.Lookup,
			}
		}
	case *SchemaData:
		if value != nil {
			shared.Value = &sharedeval.SchemaData{
				SchemaType: value.SchemaType,
				Schema:     value.Schema,
			}
		}
	default:
		shared.Value = v.Value
	}

	return shared
}

func weightedValuesDataToShared(wv *WeightedValuesData) *sharedeval.WeightedValuesData {
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

func sharedValueToLocal(shared sharedeval.Value) Value {
	value := Value{
		Type:         ValueType(shared.Type),
		Confidential: shared.Confidential,
		DecryptWith:  shared.DecryptWith,
	}

	switch raw := shared.Value.(type) {
	case *sharedeval.WeightedValuesData:
		if raw != nil {
			value.Value = sharedWeightedValuesDataToLocal(raw)
		}
	case *sharedeval.ProvidedData:
		if raw != nil {
			value.Value = &ProvidedData{
				Source: raw.Source,
				Lookup: raw.Lookup,
			}
		}
	case *sharedeval.SchemaData:
		if raw != nil {
			value.Value = &SchemaData{
				SchemaType: raw.SchemaType,
				Schema:     raw.Schema,
			}
		}
	default:
		value.Value = shared.Value
	}

	return value
}

func sharedWeightedValuesDataToLocal(shared *sharedeval.WeightedValuesData) *WeightedValuesData {
	value := &WeightedValuesData{
		HashByPropertyName: shared.HashByPropertyName,
		WeightedValues:     make([]WeightedValue, len(shared.WeightedValues)),
	}
	for i, entry := range shared.WeightedValues {
		value.WeightedValues[i] = WeightedValue{
			Weight: entry.Weight,
			Value:  sharedValueToLocal(entry.Value),
		}
	}
	return value
}
