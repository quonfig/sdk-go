package eval

import (
	quonfig "github.com/quonfig/sdk-go"
)

// ConfigResponseEvaluator wraps an Evaluator to work with ConfigResponse objects.
// It implements the quonfig.configEvaluator interface (unexported, matched by method).
type ConfigResponseEvaluator struct {
	evaluator *Evaluator
}

// NewConfigResponseEvaluator creates a new ConfigResponseEvaluator.
// The configStore is used for segment resolution during evaluation.
func NewConfigResponseEvaluator(configStore ConfigStoreGetter) *ConfigResponseEvaluator {
	return &ConfigResponseEvaluator{
		evaluator: NewEvaluator(configStore),
	}
}

// EvaluateConfigResponse evaluates a ConfigResponse for the given environment and context.
// Returns the matched value (or nil if no match).
func (e *ConfigResponseEvaluator) EvaluateConfigResponse(cfg *quonfig.ConfigResponse, envID string, ctx *quonfig.ContextSet) *quonfig.Value {
	fullCfg := ConfigResponseToFullConfig(cfg)
	var ctxGetter ContextValueGetter
	if ctx != nil {
		ctxGetter = ctx
	}
	match := e.evaluator.EvaluateConfig(fullCfg, envID, ctxGetter)
	if match.IsMatch && match.Value != nil {
		return match.Value
	}
	return nil
}

// ConfigResponseToFullConfig converts a ConfigResponse (with optional single environment)
// to a FullConfig (with environments array) for use with the evaluator.
func ConfigResponseToFullConfig(cr *quonfig.ConfigResponse) *FullConfig {
	fc := &FullConfig{
		ID:              cr.ID,
		Key:             cr.Key,
		Type:            cr.Type,
		ValueType:       cr.ValueType,
		SendToClientSDK: cr.SendToClientSDK,
		Default:         cr.Default,
	}
	if cr.Environment != nil {
		fc.Environments = []quonfig.Environment{*cr.Environment}
	}
	return fc
}

// ConfigStoreAdapter adapts a function to the ConfigStoreGetter interface.
type ConfigStoreAdapter struct {
	GetFn func(key string) (*quonfig.ConfigResponse, bool)
}

func (a *ConfigStoreAdapter) GetConfig(key string) (*FullConfig, bool) {
	cfg, ok := a.GetFn(key)
	if !ok {
		return nil, false
	}
	return ConfigResponseToFullConfig(cfg), true
}
