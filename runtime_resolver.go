package quonfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/quonfig/sdk-go/v2/internal/qf1"
)

type runtimeResolver struct {
	envLookup EnvLookupFunc
	store     configStore
	secrets   *qf1.Reader
}

// newRuntimeResolver builds the resolver for one installed store. secrets is
// the client's qf1 reader; it outlives installs so its keyset cache does too.
func newRuntimeResolver(store configStore, secrets *qf1.Reader, envLookup EnvLookupFunc) *runtimeResolver {
	if envLookup == nil {
		envLookup = os.LookupEnv
	}
	if secrets == nil {
		secrets = qf1.NewReader(qf1.LookupFunc(envLookup))
	}
	return &runtimeResolver{
		envLookup: envLookup,
		store:     store,
		secrets:   secrets,
	}
}

func (r *runtimeResolver) ResolveValue(val *Value, configKey string, valueType ValueType, envID string, ctx *ContextSet) (*Value, error) {
	if val == nil {
		return nil, nil
	}
	// qf1 spec 13.3 R1: this SDK has no decryptWith code. A value that still
	// carries one fails like any undecryptable value and is never returned as
	// its stored (ciphertext) string.
	if val.DecryptWith != "" {
		return nil, fmt.Errorf("%w: config %q: value uses decryptWith (the old secrets format), which this SDK no longer reads; store it in secrets/ as a qf1 secret", ErrUnableToDecrypt, configKey)
	}
	if r.isSecret(configKey) {
		return r.resolveSecret(val, configKey)
	}
	if val.Type == ValueTypeProvided {
		return r.resolveProvided(val, configKey, valueType)
	}
	return val, nil
}

func (r *runtimeResolver) isSecret(configKey string) bool {
	if r.store == nil {
		return false
	}
	cfg, ok := r.store.Get(configKey)
	return ok && cfg.Type == ConfigTypeSecret
}

// resolveSecret decrypts a value of a secret document lazily, on get (qf1
// spec 8). A failure fails only this get (9.3) and wraps ErrUnableToDecrypt;
// the qf1 error text names keyset and fingerprints, never key material, the
// payload or the plaintext (9.2).
func (r *runtimeResolver) resolveSecret(val *Value, configKey string) (*Value, error) {
	stored, ok := val.Value.(string)
	if !ok || val.Type != ValueTypeString {
		return nil, fmt.Errorf("%w: config %q: secret value is not a qf1 string", ErrUnableToDecrypt, configKey)
	}
	plain, err := r.secrets.Decrypt(stored)
	if err != nil {
		return nil, fmt.Errorf("%w: config %q: %w", ErrUnableToDecrypt, configKey, err)
	}
	return &Value{
		Type:         ValueTypeString,
		Value:        plain,
		Confidential: true,
	}, nil
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
		// Never echo envValue (or a parse error that quotes it): env vars carry
		// secrets, and this error reaches getter callers, logs and OpenFeature
		// error events. Name the variable, the type and the config instead.
		return nil, fmt.Errorf("%w: environment variable %q cannot be converted to %s for config %q: %v", ErrUnableToCoerce, provided.Lookup, valueType, configKey, err)
	}

	return &Value{
		Type:  valueTypeToType(valueType),
		Value: coerced,
	}, nil
}

// coerceValue converts an env var string to valueType. Its errors give a
// reason only and never contain value (see resolveProvided).
func coerceValue(value string, valueType ValueType) (interface{}, error) {
	switch valueType {
	case ValueTypeString, "":
		return value, nil
	case ValueTypeInt:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing int: %w", numErrReason(err))
		}
		return parsed, nil
	case ValueTypeDouble:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing double: %w", numErrReason(err))
		}
		return parsed, nil
	case ValueTypeBool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("parsing bool: %w", numErrReason(err))
		}
		return parsed, nil
	case ValueTypeStringList:
		// Comma-separated, each item trimmed; an empty variable is an empty list.
		if value == "" {
			return []string{}, nil
		}
		parts := strings.Split(value, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		return parts, nil
	case ValueTypeJSON:
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			// A json.SyntaxError can quote a character of the input.
			return nil, errors.New("parsing json: not valid JSON")
		}
		return parsed, nil
	case ValueTypeDuration:
		// Validated here so a malformed env var fails at resolution like any
		// other uncoercible value; the ISO string is kept for the getter.
		if _, err := ParseISO8601Duration(value); err != nil {
			// ParseISO8601Duration's error quotes its input.
			return nil, errors.New("parsing duration: not a valid ISO 8601 duration")
		}
		return value, nil
	default:
		return value, nil
	}
}

// numErrReason strips a *strconv.NumError down to its reason (ErrSyntax or
// ErrRange): NumError.Error() quotes the input it failed to parse.
func numErrReason(err error) error {
	var ne *strconv.NumError
	if errors.As(err, &ne) {
		return ne.Err
	}
	return err
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
	case ValueTypeJSON:
		return ValueTypeJSON
	case ValueTypeDuration:
		return ValueTypeDuration
	default:
		return ValueTypeString
	}
}
