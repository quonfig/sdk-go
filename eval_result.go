package quonfig

import (
	"fmt"
	"strings"
)

// EvalReason describes why a particular value was returned from evaluation.
// Maps to OpenFeature evaluation reasons and the telemetry payload's reason field.
type EvalReason int

const (
	// ReasonUnknown is the zero value; used when reason is not determined.
	ReasonUnknown EvalReason = 0
	// ReasonStatic means the config has no targeting rules -- just a static value.
	ReasonStatic EvalReason = 1
	// ReasonTargetingMatch means a rule's criteria matched the evaluation context.
	ReasonTargetingMatch EvalReason = 2
	// ReasonSplit means a weighted value (A/B test) was resolved.
	ReasonSplit EvalReason = 3
	// ReasonDefault means the SDK-provided default was returned (no match or error).
	ReasonDefault EvalReason = 4
	// ReasonError means evaluation failed (type mismatch, missing config, etc.).
	ReasonError EvalReason = 5
)

// String returns the OpenFeature-compatible reason string.
func (r EvalReason) String() string {
	switch r {
	case ReasonStatic:
		return "STATIC"
	case ReasonTargetingMatch:
		return "TARGETING_MATCH"
	case ReasonSplit:
		return "SPLIT"
	case ReasonDefault:
		return "DEFAULT"
	case ReasonError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ErrorCode is a typed enumeration of evaluation error categories. Values mirror
// the OpenFeature spec's error codes so providers can forward them without
// inferring meaning from error message text.
type ErrorCode string

const (
	// ErrorCodeNone indicates a successful evaluation (no error).
	ErrorCodeNone ErrorCode = ""
	// ErrorCodeFlagNotFound indicates the requested key is not present in the store.
	ErrorCodeFlagNotFound ErrorCode = "FLAG_NOT_FOUND"
	// ErrorCodeTypeMismatch indicates a value could not be coerced to the requested type.
	ErrorCodeTypeMismatch ErrorCode = "TYPE_MISMATCH"
	// ErrorCodeProviderNotReady indicates the SDK has not finished initialization.
	ErrorCodeProviderNotReady ErrorCode = "PROVIDER_NOT_READY"
	// ErrorCodeGeneral covers errors that don't fit the more specific categories
	// (missing env vars, decryption failures, etc.).
	ErrorCodeGeneral ErrorCode = "GENERAL"
)

// EvalResult is the internal result of evaluating a config, carrying full metadata
// needed for telemetry reporting and (future) OpenFeature evaluation details.
type EvalResult struct {
	Value              *Value
	ConfigID           string
	ConfigKey          string
	ConfigType         ConfigType
	RuleIndex          int
	WeightedValueIndex int
	Reason             EvalReason
	IsMatch            bool
}

// EvaluationDetails is the public, OpenFeature-shaped record of an evaluation.
// It bundles the resolved Value with the reason, typed error code, variant, and
// flagMetadata so callers (and OpenFeature providers) can populate
// ProviderResolutionDetail without inferring fields from error message text.
type EvaluationDetails struct {
	// Value is the resolved value, or nil on error / not-found.
	Value *Value
	// Reason describes why this value was returned.
	Reason EvalReason
	// ErrorCode is the typed error category. Empty (ErrorCodeNone) on success.
	ErrorCode ErrorCode
	// ErrorMessage is a human-readable error description. Empty on success.
	ErrorMessage string
	// Variant is the OpenFeature-style variant identifier (e.g. "static",
	// "targeting:0", "split:1", "default"). Always set, never empty.
	Variant string
	// FlagMetadata carries provider-specific data (configId, configType,
	// environment, ruleIndex, weightedValueIndex). Always non-nil; keys
	// follow camelCase (Go idiom) per the cross-SDK spec.
	FlagMetadata map[string]any
}

// variantFor formats the variant string per the cross-SDK spec
// (project/plans/openfeature-resolution-details.md §2).
func variantFor(reason EvalReason, ruleIndex, weightedIndex int) string {
	switch reason {
	case ReasonStatic:
		return "static"
	case ReasonTargetingMatch:
		return fmt.Sprintf("targeting:%d", ruleIndex)
	case ReasonSplit:
		return fmt.Sprintf("split:%d", weightedIndex)
	default:
		return "default"
	}
}

// configTypeUpper returns the SHOUTY_SNAKE form of a ConfigType (the camelCase
// idiom: spec §3 requires uppercase configType in node/go/java SDKs).
func configTypeUpper(t ConfigType) string {
	if t == "" {
		return ""
	}
	return strings.ToUpper(string(t))
}

// flagMetadataFor builds the public flagMetadata map from an EvalResult. Keys
// are omitted when not applicable (never set to null/-1) per the spec.
func flagMetadataFor(result *EvalResult, envID string) map[string]any {
	md := make(map[string]any)
	if result == nil {
		return md
	}
	if result.ConfigID != "" {
		md["configId"] = result.ConfigID
	}
	if upper := configTypeUpper(result.ConfigType); upper != "" {
		md["configType"] = upper
	}
	if envID != "" {
		md["environment"] = envID
	}
	if result.Reason == ReasonTargetingMatch || result.Reason == ReasonSplit {
		md["ruleIndex"] = int64(result.RuleIndex)
	}
	if result.Reason == ReasonSplit {
		md["weightedValueIndex"] = int64(result.WeightedValueIndex)
	}
	return md
}
