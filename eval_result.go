package quonfig

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
