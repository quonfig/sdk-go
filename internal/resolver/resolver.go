// Package resolver handles post-evaluation value resolution including environment
// variable lookup, type coercion, and decryption of confidential values.
package resolver

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"

	quonfig "github.com/quonfig/sdk-go"
	"github.com/quonfig/sdk-go/internal/encryption"
	"github.com/quonfig/sdk-go/internal/eval"
)

// ReportableValuePrefix is the redaction prefix for confidential values in
// telemetry payloads. The full form is "*****<first-5-hex-chars-of-md5(raw)>".
const ReportableValuePrefix = "*****"

// ReportableValueFor returns the redacted form of a value for telemetry
// reporting, or nil if the value is not confidential and does not require
// decryption. The hash is computed over the raw stored string value (the
// ciphertext for decryptWith values, the plaintext for plain confidential
// values) -- not over the decrypted plaintext.
func ReportableValueFor(val *quonfig.Value) *string {
	if val == nil {
		return nil
	}
	if !val.Confidential && val.DecryptWith == "" {
		return nil
	}
	raw := val.StringValue()
	sum := md5.Sum([]byte(raw))
	h := hex.EncodeToString(sum[:])
	if len(h) < 5 {
		return nil
	}
	out := ReportableValuePrefix + h[:5]
	return &out
}

// Error types for resolver failures.
var (
	ErrMissingEnvVar     = errors.New("missing_env_var")
	ErrUnableToCoerce    = errors.New("unable_to_coerce_env_var")
	ErrUnableToDecrypt   = errors.New("unable_to_decrypt")
	ErrMissingDefault    = errors.New("missing_default")
)

// EnvLookup is a function that looks up environment variables.
// It returns the value and whether it was found.
type EnvLookup func(key string) (string, bool)

// DefaultEnvLookup uses os.LookupEnv.
func DefaultEnvLookup(key string) (string, bool) {
	return os.LookupEnv(key)
}

// ConfigGetter can look up configs by key and evaluate them.
// This is used for resolving decryption key configs.
type ConfigGetter interface {
	GetConfig(key string) (*eval.FullConfig, bool)
}

// Resolver handles post-evaluation value resolution.
type Resolver struct {
	envLookup   EnvLookup
	configStore ConfigGetter
	evaluator   *eval.Evaluator
}

// New creates a new Resolver with the given dependencies.
func New(configStore ConfigGetter, evaluator *eval.Evaluator, envLookup EnvLookup) *Resolver {
	if envLookup == nil {
		envLookup = DefaultEnvLookup
	}
	return &Resolver{
		envLookup:   envLookup,
		configStore: configStore,
		evaluator:   evaluator,
	}
}

// Resolve takes an evaluated value and config metadata, and resolves it to its final form.
// This handles:
// - ENV_VAR provided values (lookup + coercion)
// - Confidential/encrypted values (decryption)
// - Pass-through for all other values
func (r *Resolver) Resolve(val *quonfig.Value, cfg *eval.FullConfig, envID string, ctx eval.ContextValueGetter) (*quonfig.Value, error) {
	if val == nil {
		return nil, nil
	}

	// Handle provided values (ENV_VAR)
	if val.Type == quonfig.ValueTypeProvided {
		return r.resolveProvided(val, cfg)
	}

	// Handle confidential/encrypted values
	if val.Confidential && val.DecryptWith != "" {
		return r.resolveDecryption(val, cfg, envID, ctx)
	}

	// Pass through
	return val, nil
}

// resolveProvided handles ENV_VAR provided values.
func (r *Resolver) resolveProvided(val *quonfig.Value, cfg *eval.FullConfig) (*quonfig.Value, error) {
	pd := val.ProvidedValue()
	if pd == nil {
		return val, nil
	}

	if pd.Source != "ENV_VAR" {
		return val, nil
	}

	envValue, exists := r.envLookup(pd.Lookup)
	if !exists {
		return nil, fmt.Errorf("%w: environment variable %q not set for config %q", ErrMissingEnvVar, pd.Lookup, cfg.Key)
	}

	// Coerce the string value to the config's declared valueType
	coerced, err := coerceValue(envValue, cfg.ValueType)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot convert %q to %s for config %q: %v", ErrUnableToCoerce, envValue, cfg.ValueType, cfg.Key, err)
	}

	return &quonfig.Value{
		Type:  valueTypeToType(cfg.ValueType),
		Value: coerced,
	}, nil
}

// resolveDecryption handles confidential values that need decryption.
func (r *Resolver) resolveDecryption(val *quonfig.Value, cfg *eval.FullConfig, envID string, ctx eval.ContextValueGetter) (*quonfig.Value, error) {
	if r.configStore == nil || r.evaluator == nil {
		return nil, fmt.Errorf("%w: no config store available for decryption key lookup", ErrUnableToDecrypt)
	}

	// Look up the decryption key config
	keyCfg, exists := r.configStore.GetConfig(val.DecryptWith)
	if !exists {
		return nil, fmt.Errorf("%w: decryption key config %q not found", ErrUnableToDecrypt, val.DecryptWith)
	}

	// Evaluate the key config to get its value
	keyMatch := r.evaluator.EvaluateConfig(keyCfg, envID, ctx)
	if !keyMatch.IsMatch || keyMatch.Value == nil {
		return nil, fmt.Errorf("%w: decryption key config %q did not match", ErrUnableToDecrypt, val.DecryptWith)
	}

	// The key config value might itself be a provided value (ENV_VAR), so resolve it recursively
	resolvedKey, err := r.Resolve(keyMatch.Value, keyCfg, envID, ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to resolve decryption key from %q: %v", ErrUnableToDecrypt, val.DecryptWith, err)
	}

	secretKey := resolvedKey.StringValue()
	if secretKey == "" {
		return nil, fmt.Errorf("%w: decryption key from %q is empty", ErrUnableToDecrypt, val.DecryptWith)
	}

	// Decrypt the value
	encryptedValue := val.StringValue()
	decrypted, err := encryption.DecryptValue(secretKey, encryptedValue)
	if err != nil {
		return nil, fmt.Errorf("%w: decryption failed for config %q: %v", ErrUnableToDecrypt, cfg.Key, err)
	}

	return &quonfig.Value{
		Type:         quonfig.ValueTypeString,
		Value:        decrypted,
		Confidential: true,
	}, nil
}

// coerceValue converts a string to the target type.
func coerceValue(value string, valueType quonfig.ValueType) (interface{}, error) {
	switch valueType {
	case quonfig.ValueTypeString, "":
		return value, nil
	case quonfig.ValueTypeInt:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing int: %w", err)
		}
		return i, nil
	case quonfig.ValueTypeDouble:
		f, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("parsing double: %w", err)
		}
		return f, nil
	case quonfig.ValueTypeBool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("parsing bool: %w", err)
		}
		return b, nil
	default:
		return value, nil
	}
}

// valueTypeToType converts a ValueType (the config's declared type) to the Value.Type field.
func valueTypeToType(vt quonfig.ValueType) quonfig.ValueType {
	switch vt {
	case quonfig.ValueTypeInt:
		return quonfig.ValueTypeInt
	case quonfig.ValueTypeDouble:
		return quonfig.ValueTypeDouble
	case quonfig.ValueTypeBool:
		return quonfig.ValueTypeBool
	case quonfig.ValueTypeStringList:
		return quonfig.ValueTypeStringList
	default:
		return quonfig.ValueTypeString
	}
}
