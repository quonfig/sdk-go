package fixtures

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	quonfig "github.com/quonfig/sdk-go"
	"github.com/quonfig/sdk-go/internal/eval"
	"github.com/quonfig/sdk-go/internal/resolver"
	"github.com/quonfig/sdk-go/internal/telemetry"
)

const (
	dataDir = "../../../integration-test-data/data/integration-tests"
)

var (
	configStore  *ConfigStore
	evaluator    *eval.Evaluator
	testResolver *resolver.Resolver
)

// testEnvVars are the environment variables set for integration tests.
// These simulate the environment that the SDK would run in.
var testEnvVars = map[string]string{
	"PREFAB_INTEGRATION_TEST_ENCRYPTION_KEY": "c87ba22d8662282abe8a0e4651327b579cb64a454ab0f4c170b45b15f049a221",
	"IS_A_NUMBER": "1234",
	"NOT_A_NUMBER": "not_a_number",
	// MISSING_ENV_VAR is intentionally NOT set
}

func TestMain(m *testing.M) {
	configStore = NewConfigStore()
	if err := configStore.LoadFromDir(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config data: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d configs from %s\n", configStore.Len(), dataDir)

	evaluator = eval.NewEvaluator(configStore)

	// Create a resolver with a test env lookup that uses our test env vars
	envLookup := func(key string) (string, bool) {
		val, ok := testEnvVars[key]
		return val, ok
	}
	testResolver = resolver.New(configStore, evaluator, envLookup)

	os.Exit(m.Run())
}

// buildContextFromMaps builds a merged ContextSet from three context levels.
// Each level is a map of context-name -> map of property-name -> value.
func buildContextFromMaps(global, block, local map[string]map[string]interface{}) eval.ContextValueGetter {
	globalCtx := buildContextSet(global)
	blockCtx := buildContextSet(block)
	localCtx := buildContextSet(local)

	if globalCtx == nil && blockCtx == nil && localCtx == nil {
		return eval.EmptyContext{}
	}

	return quonfig.Merge(globalCtx, blockCtx, localCtx)
}

func buildContextSet(contextMap map[string]map[string]interface{}) *quonfig.ContextSet {
	if contextMap == nil {
		return nil
	}

	cs := quonfig.NewContextSet()
	for name, values := range contextMap {
		cs.WithNamedContextValues(name, values)
	}
	return cs
}

// mustLookupConfig looks up a config by key, failing the test if not found.
func mustLookupConfig(t *testing.T, key string) *eval.FullConfig {
	t.Helper()
	cfg, ok := configStore.GetConfig(key)
	if !ok {
		t.Fatalf("config not found: %s", key)
	}
	return cfg
}

// evaluateAndResolve evaluates a config and resolves the result through the resolver.
func evaluateAndResolve(t *testing.T, cfg *eval.FullConfig, ctx eval.ContextValueGetter) (*eval.EvalMatch, error) {
	t.Helper()
	match := evaluator.EvaluateConfig(cfg, "Production", ctx)

	if match.IsMatch && match.Value != nil {
		resolved, err := testResolver.Resolve(match.Value, cfg, "Production", ctx)
		if err != nil {
			return match, err
		}
		match.Value = resolved
	}

	return match, nil
}

// assertStringValue asserts that the resolved value is the expected string.
func assertStringValue(t *testing.T, match *eval.EvalMatch, expected string) {
	t.Helper()
	if !match.IsMatch {
		t.Fatalf("expected string %q but got no match", expected)
	}
	got := match.Value.StringValue()
	if got != expected {
		t.Errorf("expected string %q but got %q", expected, got)
	}
}

// assertIntValue asserts that the resolved value is the expected int.
func assertIntValue(t *testing.T, match *eval.EvalMatch, expected int64) {
	t.Helper()
	if !match.IsMatch {
		t.Fatalf("expected int %d but got no match", expected)
	}
	got := match.Value.IntValue()
	if got != expected {
		t.Errorf("expected int %d but got %d", expected, got)
	}
}

// assertDoubleValue asserts that the resolved value is the expected float.
func assertDoubleValue(t *testing.T, match *eval.EvalMatch, expected float64) {
	t.Helper()
	if !match.IsMatch {
		t.Fatalf("expected double %f but got no match", expected)
	}
	got := match.Value.DoubleValue()
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected double %f but got %f", expected, got)
	}
}

// assertBoolValue asserts that the resolved value is the expected bool.
func assertBoolValue(t *testing.T, match *eval.EvalMatch, expected bool) {
	t.Helper()
	if !match.IsMatch {
		if expected {
			t.Errorf("expected bool true but got no match")
		}
		return
	}
	got := match.Value.BoolValue()
	if got != expected {
		t.Errorf("expected bool %v but got %v", expected, got)
	}
}

// assertEnabledValue asserts the enabled result. For enabled, no-match = false,
// and non-boolean flags always return false.
func assertEnabledValue(t *testing.T, match *eval.EvalMatch, expected bool) {
	t.Helper()
	if !match.IsMatch {
		if expected {
			t.Errorf("expected enabled=true but got no match")
		}
		return
	}
	got := match.Value.BoolValue()
	if got != expected {
		t.Errorf("expected enabled=%v but got %v", expected, got)
	}
}

// assertStringListValue asserts that the resolved value is the expected string list.
func assertStringListValue(t *testing.T, match *eval.EvalMatch, expected []string) {
	t.Helper()
	if !match.IsMatch {
		t.Fatalf("expected string list %v but got no match", expected)
	}
	got := match.Value.StringListValue()
	if len(got) != len(expected) {
		t.Fatalf("expected string list %v (len %d) but got %v (len %d)", expected, len(expected), got, len(got))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("expected string list[%d] = %q but got %q", i, expected[i], got[i])
		}
	}
}

// assertJSONValue asserts that the resolved value is a native JSON object
// containing the expected fields. Values must be stored as native JSON on the
// Value (map[string]interface{} / []interface{} / number / bool / nil),
// never as a stringified payload.
func assertJSONValue(t *testing.T, match *eval.EvalMatch, expected map[string]interface{}) {
	t.Helper()
	if !match.IsMatch {
		t.Fatalf("expected JSON value but got no match")
	}
	if s, isString := match.Value.Value.(string); isString {
		t.Fatalf("expected native JSON object but got stringified payload %q", s)
	}
	parsed, ok := match.Value.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("expected JSON object (map[string]interface{}) but got %T: %v", match.Value.Value, match.Value.Value)
	}
	for k, v := range expected {
		pv, ok := parsed[k]
		if !ok {
			t.Errorf("expected JSON key %q not found in %v", k, parsed)
			continue
		}
		// Compare as strings for simplicity since JSON numbers are float64
		if fmt.Sprintf("%v", pv) != fmt.Sprintf("%v", v) {
			t.Errorf("expected JSON[%q] = %v but got %v", k, v, pv)
		}
	}
}

// assertDurationMillis asserts that the resolved value is a duration with the expected milliseconds.
func assertDurationMillis(t *testing.T, match *eval.EvalMatch, expectedMillis int64) {
	t.Helper()
	if !match.IsMatch {
		t.Fatalf("expected duration but got no match")
	}
	durationStr := match.Value.StringValue()
	millis, err := parseISO8601Duration(durationStr)
	if err != nil {
		t.Fatalf("failed to parse duration %q: %v", durationStr, err)
	}
	if math.Abs(float64(millis)-float64(expectedMillis)) > 1 {
		t.Errorf("expected %d millis but got %d (from duration %q)", expectedMillis, millis, durationStr)
	}
}

// assertNilValue asserts that the evaluation produced no match.
func assertNilValue(t *testing.T, match *eval.EvalMatch) {
	t.Helper()
	if match.IsMatch {
		t.Errorf("expected nil/no match but got a match with value %v", match.Value.Value)
	}
}

// assertDefaultStringValue is used when a missing config should return a default value.
func assertDefaultStringValue(t *testing.T, key string, defaultVal string, expected string) {
	t.Helper()
	_, ok := configStore.GetConfig(key)
	if ok {
		t.Fatalf("expected config %q to be missing (testing default), but it was found", key)
	}
	if defaultVal != expected {
		t.Errorf("expected default %q but got %q", expected, defaultVal)
	}
}

// assertResolveError asserts that resolution produced the expected error type.
func assertResolveError(t *testing.T, err error, expectedError string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected raise with error %q but got no error", expectedError)
	}
	switch expectedError {
	case "missing_env_var":
		if !errors.Is(err, resolver.ErrMissingEnvVar) {
			t.Errorf("expected missing_env_var error, got: %v", err)
		}
	case "unable_to_coerce_env_var":
		if !errors.Is(err, resolver.ErrUnableToCoerce) {
			t.Errorf("expected unable_to_coerce_env_var error, got: %v", err)
		}
	case "unable_to_decrypt":
		if !errors.Is(err, resolver.ErrUnableToDecrypt) {
			t.Errorf("expected unable_to_decrypt error, got: %v", err)
		}
	default:
		t.Errorf("unknown expected error type: %q, got: %v", expectedError, err)
	}
}

// parseISO8601Duration parses an ISO 8601 duration string and returns milliseconds.
// Supports: P[n]DT[n]H[n]M[n]S (e.g., PT0.2S, PT90S, PT1.5M, PT0.5H, P1DT6H2M1.5S)
func parseISO8601Duration(s string) (int64, error) {
	if len(s) < 2 || s[0] != 'P' {
		return 0, fmt.Errorf("invalid ISO 8601 duration: %s", s)
	}

	var totalMillis float64
	i := 1 // skip 'P'
	inTimePart := false

	for i < len(s) {
		if s[i] == 'T' {
			inTimePart = true
			i++
			continue
		}

		// Parse the number
		start := i
		for i < len(s) && (s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
			i++
		}
		if i >= len(s) {
			return 0, fmt.Errorf("invalid ISO 8601 duration: unexpected end: %s", s)
		}

		numStr := s[start:i]
		var num float64
		if _, err := fmt.Sscanf(numStr, "%f", &num); err != nil {
			return 0, fmt.Errorf("invalid number in duration %q: %w", numStr, err)
		}

		unit := s[i]
		i++

		if inTimePart {
			switch unit {
			case 'H':
				totalMillis += num * 3600000
			case 'M':
				totalMillis += num * 60000
			case 'S':
				totalMillis += num * 1000
			default:
				return 0, fmt.Errorf("unknown time unit %c in duration %s", unit, s)
			}
		} else {
			switch unit {
			case 'Y':
				totalMillis += num * 365.25 * 86400000
			case 'M':
				totalMillis += num * 30 * 86400000
			case 'W':
				totalMillis += num * 7 * 86400000
			case 'D':
				totalMillis += num * 86400000
			default:
				return 0, fmt.Errorf("unknown date unit %c in duration %s", unit, s)
			}
		}
	}

	return int64(math.Round(totalMillis)), nil
}

// configTypeToTelemetryType converts a quonfig.ConfigType to the uppercase format used in telemetry payloads.
func configTypeToTelemetryType(ct quonfig.ConfigType) string {
	switch ct {
	case quonfig.ConfigTypeConfig:
		return "CONFIG"
	case quonfig.ConfigTypeFeatureFlag:
		return "FEATURE_FLAG"
	case quonfig.ConfigTypeSegment:
		return "SEGMENT"
	default:
		return strings.ToUpper(strings.ReplaceAll(string(ct), "_", "_"))
	}
}

// evaluateForTelemetry evaluates a config and returns a telemetry.EvalMatch suitable for recording.
func evaluateForTelemetry(t *testing.T, key string, ctx eval.ContextValueGetter) telemetry.EvalMatch {
	t.Helper()
	cfg := mustLookupConfig(t, key)
	match := evaluator.EvaluateConfig(cfg, "Production", ctx)

	var selectedValue interface{}
	reason := 4 // DEFAULT

	if match.IsMatch && match.Value != nil {
		// Resolve value
		resolved, err := testResolver.Resolve(match.Value, cfg, "Production", ctx)
		if err != nil {
			t.Fatalf("resolve failed for %s: %v", key, err)
		}
		selectedValue = resolved.Value

		// Determine reason (same logic as runtime_eval.go)
		switch {
		case match.WeightedValueIndex > 0:
			reason = 3 // SPLIT
		case !hasTargetingRules(cfg):
			reason = 1 // STATIC
		default:
			reason = 2 // TARGETING_MATCH
		}
	}

	return telemetry.EvalMatch{
		ConfigID:           cfg.ID,
		ConfigKey:          cfg.Key,
		ConfigType:         configTypeToTelemetryType(cfg.Type),
		RuleIndex:          match.RuleIndex,
		WeightedValueIndex: match.WeightedValueIndex,
		SelectedValue:      selectedValue,
		Reason:             reason,
	}
}

// hasTargetingRules returns true if any rule has non-ALWAYS_TRUE criteria.
func hasTargetingRules(cfg *eval.FullConfig) bool {
	checkRules := func(rules []quonfig.Rule) bool {
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
	for _, env := range cfg.Environments {
		if checkRules(env.Rules) {
			return true
		}
	}
	return false
}

// assertEvalSummaryCounter checks that a specific counter exists in the eval summary event.
func assertEvalSummaryCounter(t *testing.T, event *telemetry.TelemetryEvent, expectedKey, expectedType string, expectedCount int64, expectedReason int) {
	t.Helper()
	if event == nil || event.Summaries == nil {
		t.Fatalf("expected eval summary event, got nil")
	}
	for _, summary := range event.Summaries.Summaries {
		if summary.Key == expectedKey && summary.Type == expectedType {
			for _, counter := range summary.Counters {
				if counter.Count == expectedCount && counter.Reason == expectedReason {
					return // found
				}
			}
		}
	}
	t.Errorf("counter not found: key=%s type=%s count=%d reason=%d", expectedKey, expectedType, expectedCount, expectedReason)
}

// assertEvalSummaryCounterFull checks that a specific counter exists with detailed field matching.
func assertEvalSummaryCounterFull(t *testing.T, event *telemetry.TelemetryEvent, expectedKey, expectedType string, expectedCount int64, expectedReason int, expectedConfigRowIndex int, expectedConditionalValueIndex int, expectedWeightedValueIndex int) {
	t.Helper()
	if event == nil || event.Summaries == nil {
		t.Fatalf("expected eval summary event, got nil")
	}
	for _, summary := range event.Summaries.Summaries {
		if summary.Key == expectedKey && summary.Type == expectedType {
			for _, counter := range summary.Counters {
				if counter.Count == expectedCount &&
					counter.Reason == expectedReason &&
					counter.ConfigRowIndex == expectedConfigRowIndex &&
					counter.ConditionalValueIndex == expectedConditionalValueIndex &&
					counter.WeightedValueIndex == expectedWeightedValueIndex {
					return // found
				}
			}
		}
	}
	t.Errorf("counter not found: key=%s type=%s count=%d reason=%d configRowIndex=%d conditionalValueIndex=%d weightedValueIndex=%d",
		expectedKey, expectedType, expectedCount, expectedReason, expectedConfigRowIndex, expectedConditionalValueIndex, expectedWeightedValueIndex)
}
