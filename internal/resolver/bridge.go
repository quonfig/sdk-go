package resolver

import (
	quonfig "github.com/quonfig/sdk-go"
	"github.com/quonfig/sdk-go/internal/eval"
)

// ClientResolver wraps a Resolver to satisfy the quonfig.ValueResolver interface.
// It bridges between the public quonfig types and the internal eval types.
type ClientResolver struct {
	resolver *Resolver
}

// NewClientResolver creates a new ClientResolver.
func NewClientResolver(configStore ConfigGetter, evaluator *eval.Evaluator, envLookup EnvLookup) *ClientResolver {
	return &ClientResolver{
		resolver: New(configStore, evaluator, envLookup),
	}
}

// ResolveValue resolves a matched value, handling ENV_VAR provided values and decryption.
func (cr *ClientResolver) ResolveValue(val *quonfig.Value, configKey string, valueType quonfig.ValueType, envID string, ctx *quonfig.ContextSet) (*quonfig.Value, error) {
	// Build a minimal FullConfig with just the key and valueType for the resolver
	cfg := &eval.FullConfig{
		Key:       configKey,
		ValueType: valueType,
	}

	var ctxGetter eval.ContextValueGetter
	if ctx != nil {
		ctxGetter = ctx
	} else {
		ctxGetter = eval.EmptyContext{}
	}

	return cr.resolver.Resolve(val, cfg, envID, ctxGetter)
}
