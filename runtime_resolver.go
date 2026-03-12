package quonfig

import (
	"fmt"
	"os"
	"strconv"

	"github.com/quonfig/sdk-go/internal/encryption"
)

type runtimeResolver struct {
	envLookup EnvLookupFunc
	store     configStore
	evaluator *runtimeEvaluator
}

func newRuntimeResolver(store configStore, evaluator *runtimeEvaluator, envLookup EnvLookupFunc) *runtimeResolver {
	if envLookup == nil {
		envLookup = os.LookupEnv
	}
	return &runtimeResolver{
		envLookup: envLookup,
		store:     store,
		evaluator: evaluator,
	}
}

func (r *runtimeResolver) ResolveValue(val *Value, configKey string, valueType ValueType, envID string, ctx *ContextSet) (*Value, error) {
	if val == nil {
		return nil, nil
	}
	if val.Type == ValueTypeProvided {
		return r.resolveProvided(val, configKey, valueType)
	}
	if val.Confidential && val.DecryptWith != "" {
		return r.resolveDecryption(val, configKey, envID, ctx)
	}
	return val, nil
}

func (r *runtimeResolver) resolveProvided(val *Value, configKey string, valueType ValueType) (*Value, error) {
	provided := val.ProvidedValue()
	if provided == nil || provided.Source != "ENV_VAR" {
		return val, nil
	}

	envValue, exists := r.envLookup(provided.Lookup)
	if !exists {
		return nil, fmt.Errorf("%w: environment variable %q not set for config %q", ErrMissingEnvVar, provided.Lookup, configKey)
	}

	coerced, err := coerceValue(envValue, valueType)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot convert %q to %s for config %q: %v", ErrUnableToCoerce, envValue, valueType, configKey, err)
	}

	return &Value{
		Type:  valueTypeToType(valueType),
		Value: coerced,
	}, nil
}

func (r *runtimeResolver) resolveDecryption(val *Value, configKey, envID string, ctx *ContextSet) (*Value, error) {
	keyCfg, ok := r.store.Get(val.DecryptWith)
	if !ok {
		return nil, fmt.Errorf("%w: decryption key config %q not found", ErrUnableToDecrypt, val.DecryptWith)
	}

	keyValue := r.evaluator.EvaluateConfigResponse(keyCfg, envID, ctx)
	if keyValue == nil {
		return nil, fmt.Errorf("%w: decryption key config %q did not match", ErrUnableToDecrypt, val.DecryptWith)
	}

	resolvedKey, err := r.ResolveValue(keyValue, keyCfg.Key, keyCfg.ValueType, envID, ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to resolve decryption key from %q: %v", ErrUnableToDecrypt, val.DecryptWith, err)
	}

	secretKey := resolvedKey.StringValue()
	if secretKey == "" {
		return nil, fmt.Errorf("%w: decryption key from %q is empty", ErrUnableToDecrypt, val.DecryptWith)
	}

	decrypted, err := encryption.DecryptValue(secretKey, val.StringValue())
	if err != nil {
		return nil, fmt.Errorf("%w: decryption failed for config %q: %v", ErrUnableToDecrypt, configKey, err)
	}

	return &Value{
		Type:         ValueTypeString,
		Value:        decrypted,
		Confidential: true,
	}, nil
}

func coerceValue(value string, valueType ValueType) (interface{}, error) {
	switch valueType {
	case ValueTypeString, "":
		return value, nil
	case ValueTypeInt:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing int: %w", err)
		}
		return parsed, nil
	case ValueTypeDouble:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing double: %w", err)
		}
		return parsed, nil
	case ValueTypeBool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("parsing bool: %w", err)
		}
		return parsed, nil
	default:
		return value, nil
	}
}

func valueTypeToType(valueType ValueType) ValueType {
	switch valueType {
	case ValueTypeInt:
		return ValueTypeInt
	case ValueTypeDouble:
		return ValueTypeDouble
	case ValueTypeBool:
		return ValueTypeBool
	case ValueTypeStringList:
		return ValueTypeStringList
	default:
		return ValueTypeString
	}
}
