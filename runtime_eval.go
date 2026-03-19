package quonfig

import evalcore "github.com/quonfig/sdk-go/evalcore"

type runtimeEvaluator struct {
	shared *evalcore.Evaluator
}

type sharedConfigStoreAdapter struct {
	store configStore
}

func newRuntimeEvaluator(store configStore) *runtimeEvaluator {
	return &runtimeEvaluator{
		shared: evalcore.NewEvaluator(&sharedConfigStoreAdapter{store: store}),
	}
}

func (a *sharedConfigStoreAdapter) GetConfig(key string) (*evalcore.Config, bool) {
	cfg, ok := a.store.Get(key)
	if !ok {
		return nil, false
	}
	return configResponseToShared(cfg), true
}

func (e *runtimeEvaluator) EvaluateConfigResponse(cfg *ConfigResponse, envID string, ctx *ContextSet) *EvalResult {
	var getter evalcore.ContextValueGetter = evalcore.EmptyContext{}
	if ctx != nil {
		getter = ctx
	}

	match := e.shared.EvaluateConfig(configResponseToShared(cfg), envID, getter)

	result := &EvalResult{
		ConfigID:   cfg.ID,
		ConfigKey:  cfg.Key,
		ConfigType: cfg.Type,
	}

	if match == nil || !match.IsMatch || match.Value == nil {
		result.IsMatch = false
		result.Reason = ReasonDefault
		return result
	}

	value := sharedValueToLocal(*match.Value)
	result.Value = &value
	result.IsMatch = true
	result.RuleIndex = match.RuleIndex
	result.WeightedValueIndex = match.WeightedValueIndex

	// Determine evaluation reason
	switch {
	case match.WeightedValueIndex > 0:
		result.Reason = ReasonSplit
	case len(match.Value.Type) > 0 && match.RuleIndex == 0 && !hasTargetingRules(cfg):
		result.Reason = ReasonStatic
	default:
		result.Reason = ReasonTargetingMatch
	}

	return result
}

// hasTargetingRules returns true if any rule has non-ALWAYS_TRUE criteria.
func hasTargetingRules(cfg *ConfigResponse) bool {
	checkRules := func(rules []Rule) bool {
		for _, rule := range rules {
			for _, c := range rule.Criteria {
				if c.Operator != "ALWAYS_TRUE" {
					return true
				}
			}
		}
		return false
	}
	if checkRules(cfg.Default.Rules) {
		return true
	}
	if cfg.Environment != nil {
		return checkRules(cfg.Environment.Rules)
	}
	return false
}

func configResponseToShared(cfg *ConfigResponse) *evalcore.Config {
	sharedCfg := &evalcore.Config{
		ID:              cfg.ID,
		Key:             cfg.Key,
		Type:            evalcore.ConfigType(cfg.Type),
		ValueType:       evalcore.ValueType(cfg.ValueType),
		SendToClientSDK: cfg.SendToClientSDK,
		Default:         ruleSetToShared(cfg.Default),
	}
	if cfg.Environment != nil {
		sharedCfg.Environments = []evalcore.Environment{environmentToShared(*cfg.Environment)}
	}
	return sharedCfg
}

func ruleSetToShared(rs RuleSet) evalcore.RuleSet {
	shared := evalcore.RuleSet{
		Rules: make([]evalcore.Rule, len(rs.Rules)),
	}
	for i, rule := range rs.Rules {
		shared.Rules[i] = ruleToShared(rule)
	}
	return shared
}

func environmentToShared(env Environment) evalcore.Environment {
	shared := evalcore.Environment{
		ID:    env.ID,
		Rules: make([]evalcore.Rule, len(env.Rules)),
	}
	for i, rule := range env.Rules {
		shared.Rules[i] = ruleToShared(rule)
	}
	return shared
}

func ruleToShared(rule Rule) evalcore.Rule {
	shared := evalcore.Rule{
		Criteria: make([]evalcore.Criterion, len(rule.Criteria)),
		Value:    valueToShared(rule.Value),
	}
	for i, criterion := range rule.Criteria {
		shared.Criteria[i] = criterionToShared(criterion)
	}
	return shared
}

func criterionToShared(c Criterion) evalcore.Criterion {
	shared := evalcore.Criterion{
		PropertyName: c.PropertyName,
		Operator:     c.Operator,
	}
	if c.ValueToMatch != nil {
		value := valueToShared(*c.ValueToMatch)
		shared.ValueToMatch = &value
	}
	return shared
}

func valueToShared(v Value) evalcore.Value {
	shared := evalcore.Value{
		Type:         evalcore.ValueType(v.Type),
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
			shared.Value = &evalcore.ProvidedData{
				Source: value.Source,
				Lookup: value.Lookup,
			}
		}
	case *SchemaData:
		if value != nil {
			shared.Value = &evalcore.SchemaData{
				SchemaType: value.SchemaType,
				Schema:     value.Schema,
			}
		}
	default:
		shared.Value = v.Value
	}

	return shared
}

func weightedValuesDataToShared(wv *WeightedValuesData) *evalcore.WeightedValuesData {
	shared := &evalcore.WeightedValuesData{
		HashByPropertyName: wv.HashByPropertyName,
		WeightedValues:     make([]evalcore.WeightedValue, len(wv.WeightedValues)),
	}
	for i, entry := range wv.WeightedValues {
		shared.WeightedValues[i] = evalcore.WeightedValue{
			Weight: entry.Weight,
			Value:  valueToShared(entry.Value),
		}
	}
	return shared
}

func sharedValueToLocal(shared evalcore.Value) Value {
	value := Value{
		Type:         ValueType(shared.Type),
		Confidential: shared.Confidential,
		DecryptWith:  shared.DecryptWith,
	}

	switch raw := shared.Value.(type) {
	case *evalcore.WeightedValuesData:
		if raw != nil {
			value.Value = sharedWeightedValuesDataToLocal(raw)
		}
	case *evalcore.ProvidedData:
		if raw != nil {
			value.Value = &ProvidedData{
				Source: raw.Source,
				Lookup: raw.Lookup,
			}
		}
	case *evalcore.SchemaData:
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

func sharedWeightedValuesDataToLocal(shared *evalcore.WeightedValuesData) *WeightedValuesData {
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
