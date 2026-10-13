// Code generated from integration-test-data/tests/eval/telemetry.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// reason is STATIC for config with no targeting rules
func TestTelemetry_ReasonIsSTATICForConfigWithNoTargetingRules(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue("brand.new.string", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "hello.world"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// reason is STATIC for feature flag with only ALWAYS_TRUE rules
func TestTelemetry_ReasonIsSTATICForFeatureFlagWithOnlyALWAYSTRUERules(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetBoolValue("always.true", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "always.true", "type": "FEATURE_FLAG", "value": true, "value_type": "bool", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"bool": true}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// reason is TARGETING_MATCH when config has targeting rules but evaluation falls through
func TestTelemetry_ReasonIsTARGETINGMATCHWhenConfigHasTargetingRulesButEvaluationFallsThrough(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue("my-test-key", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "my-test-key", "type": "CONFIG", "value": "my-test-value", "value_type": "string", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"string": "my-test-value"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 1}}}, "/api/v1/telemetry")
}

// reason is TARGETING_MATCH when a targeting rule matches
func TestTelemetry_ReasonIsTARGETINGMATCHWhenATargetingRuleMatches(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}})).GetIntValue("feature-flag.integer")
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "feature-flag.integer", "type": "FEATURE_FLAG", "value": 5, "value_type": "int", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"int": 5}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// reason is SPLIT for weighted value evaluation
func TestTelemetry_ReasonIsSPLITForWeightedValueEvaluation(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "92a202f2"}})).GetIntValue("feature-flag.weighted")
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "feature-flag.weighted", "type": "FEATURE_FLAG", "value": 2, "value_type": "int", "count": 1, "reason": 3, "selected_value": map[string]interface{}{"int": 2}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0, "weighted_value_index": 2}}}, "/api/v1/telemetry")
}

// reason is SPLIT for weighted value landing in bucket 0
func TestTelemetry_ReasonIsSPLITForWeightedValueLandingInBucket0(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "3e9459d6"}})).GetIntValue("feature-flag.weighted")
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "feature-flag.weighted", "type": "FEATURE_FLAG", "value": 1, "value_type": "int", "count": 1, "reason": 3, "selected_value": map[string]interface{}{"int": 1}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0, "weighted_value_index": 0}}}, "/api/v1/telemetry")
}

// reason is TARGETING_MATCH for feature flag fallthrough with targeting rules
func TestTelemetry_ReasonIsTARGETINGMATCHForFeatureFlagFallthroughWithTargetingRules(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetIntValue("feature-flag.integer", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "feature-flag.integer", "type": "FEATURE_FLAG", "value": 3, "value_type": "int", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"int": 3}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 1}}}, "/api/v1/telemetry")
}

// evaluation summary deduplicates identical evaluations
func TestTelemetry_EvaluationSummaryDeduplicatesIdenticalEvaluations(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue("brand.new.string", nil)
	_, _, _ = c.GetStringValue("brand.new.string", nil)
	_, _, _ = c.GetStringValue("brand.new.string", nil)
	_, _, _ = c.GetStringValue("brand.new.string", nil)
	_, _, _ = c.GetStringValue("brand.new.string", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 5, "reason": 1, "selected_value": map[string]interface{}{"string": "hello.world"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// evaluation summary creates separate counters for different rules of same config
func TestTelemetry_EvaluationSummaryCreatesSeparateCountersForDifferentRulesOfSameConfig(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"key": "michael"}})).GetIntValue("feature-flag.integer")
	_, _, _ = c.GetIntValue("feature-flag.integer", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "feature-flag.integer", "type": "FEATURE_FLAG", "value": 5, "value_type": "int", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"int": 5}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}, map[string]interface{}{"key": "feature-flag.integer", "type": "FEATURE_FLAG", "value": 3, "value_type": "int", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"int": 3}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 1}}}, "/api/v1/telemetry")
}

// evaluation summary groups by config key
func TestTelemetry_EvaluationSummaryGroupsByConfigKey(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue("brand.new.string", nil)
	_, _, _ = c.GetBoolValue("always.true", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "hello.world"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}, map[string]interface{}{"key": "always.true", "type": "FEATURE_FLAG", "value": true, "value_type": "bool", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"bool": true}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps string correctly
func TestTelemetry_SelectedValueWrapsStringCorrectly(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue("brand.new.string", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "hello.world"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps boolean correctly
func TestTelemetry_SelectedValueWrapsBooleanCorrectly(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetBoolValue("brand.new.boolean", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.boolean", "type": "CONFIG", "value": false, "value_type": "bool", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"bool": false}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps int correctly
func TestTelemetry_SelectedValueWrapsIntCorrectly(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetIntValue("brand.new.int", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.int", "type": "CONFIG", "value": 123, "value_type": "int", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"int": 123}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps double correctly
func TestTelemetry_SelectedValueWrapsDoubleCorrectly(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetFloatValue("brand.new.double", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.double", "type": "CONFIG", "value": 123.99, "value_type": "double", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"double": 123.99}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps string list correctly
func TestTelemetry_SelectedValueWrapsStringListCorrectly(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringSliceValue("my-string-list-key", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "my-string-list-key", "type": "CONFIG", "value": []interface{}{"a", "b", "c"}, "value_type": "string_list", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"stringList": []interface{}{"a", "b", "c"}}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// context shape merges fields across multiple records
func TestTelemetry_ContextShapeMergesFieldsAcrossMultipleRecords(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue(telemetryContextProbeKey, contextSet(map[string]map[string]interface{}{"user": {"name": "alice", "age": 30}}))
	_, _, _ = c.GetStringValue(telemetryContextProbeKey, contextSet(map[string]map[string]interface{}{"user": {"name": "bob", "score": 9.5}, "team": {"name": "engineering"}}))
	assertTelemetryPost(t, tel, "context_shape", []interface{}{map[string]interface{}{"name": "user", "field_types": map[string]interface{}{"name": 2, "age": 1, "score": 4}}, map[string]interface{}{"name": "team", "field_types": map[string]interface{}{"name": 2}}}, "/api/v1/context-shapes")
}

// example contexts deduplicates by key value
func TestTelemetry_ExampleContextsDeduplicatesByKeyValue(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue(telemetryContextProbeKey, contextSet(map[string]map[string]interface{}{"user": {"key": "user-123", "name": "alice"}}))
	_, _, _ = c.GetStringValue(telemetryContextProbeKey, contextSet(map[string]map[string]interface{}{"user": {"key": "user-123", "name": "bob"}}))
	assertTelemetryPost(t, tel, "example_contexts", map[string]interface{}{"user": map[string]interface{}{"key": "user-123", "name": "alice"}}, "/api/v1/telemetry")
}

// telemetry disabled emits nothing
func TestTelemetry_TelemetryDisabledEmitsNothing(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{"collect_evaluation_summaries": false, "context_upload_mode": ":none"})
	c := tel.Client
	_, _, _ = c.GetStringValue("brand.new.string", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", nil, "/api/v1/telemetry")
}

// shapes only mode reports shapes but not examples
func TestTelemetry_ShapesOnlyModeReportsShapesButNotExamples(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{"context_upload_mode": ":shape_only"})
	c := tel.Client
	_, _, _ = c.GetStringValue(telemetryContextProbeKey, contextSet(map[string]map[string]interface{}{"user": {"name": "alice", "key": "alice-123"}}))
	assertTelemetryPost(t, tel, "context_shape", []interface{}{map[string]interface{}{"name": "user", "field_types": map[string]interface{}{"name": 2, "key": 2}}}, "/api/v1/context-shapes")
}

// log level evaluations are excluded from telemetry
func TestTelemetry_LogLevelEvaluationsAreExcludedFromTelemetry(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue("log-level.prefab.criteria_evaluator", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", nil, "/api/v1/telemetry")
}

// empty context produces no context telemetry
func TestTelemetry_EmptyContextProducesNoContextTelemetry(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue(telemetryContextProbeKey, contextSet(nil))
	assertTelemetryPost(t, tel, "context_shape", nil, "/api/v1/context-shapes")
}

// confidential plain string is redacted in selectedValue
func TestTelemetry_ConfidentialPlainStringIsRedactedInSelectedValue(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue("confidential.new.string", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "confidential.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "*****18aa7"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// confidential encrypted string is redacted using ciphertext hash
func TestTelemetry_ConfidentialEncryptedStringIsRedactedUsingCiphertextHash(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue("a.secret.config", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "a.secret.config", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "*****936c9"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// confidential weighted variant is redacted in selectedValue
func TestTelemetry_ConfidentialWeightedVariantIsRedactedInSelectedValue(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.WithContext(contextSet(map[string]map[string]interface{}{"user": {"tracking_id": "92a202f2"}})).GetStringValue("confidential.weighted.variant")
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "confidential.weighted.variant", "type": "CONFIG", "value": "weighted-secret-variant", "value_type": "string", "count": 1, "reason": 3, "selected_value": map[string]interface{}{"string": "*****364b0"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0, "weighted_value_index": 0}}}, "/api/v1/telemetry")
}

// confidential ENV_VAR-provided value is redacted in selectedValue
func TestTelemetry_ConfidentialENVVARProvidedValueIsRedactedInSelectedValue(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetStringValue("confidential.provided.a.number", nil)
	summaries := eventsOfKind(tel.events(t), "summaries")
	checkRedactedSelectedValues(t, summaries, "confidential.provided.a.number")
	assertEvalSummaryWire(t, summaries, []interface{}{map[string]interface{}{"key": "confidential.provided.a.number", "type": "CONFIG", "value": "1234", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"redacted": true}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}})
}

// non-confidential ENV_VAR-provided value is reported as its resolved value
func TestTelemetry_NonConfidentialENVVARProvidedValueIsReportedAsItsResolvedValue(t *testing.T) {
	tel := startTelemetryClient(t, map[string]interface{}{})
	c := tel.Client
	_, _, _ = c.GetIntValue("provided.a.number", nil)
	assertTelemetryPost(t, tel, "evaluation_summary", []interface{}{map[string]interface{}{"key": "provided.a.number", "type": "CONFIG", "value": 1234, "value_type": "int", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"int": 1234}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// checkRedactedSelectedValues asserts every counter for keys reports a
// selectedValue of exactly {"string": "*****<5 hex>"} (the redaction shape;
// the hash input is not pinned), then replaces it with the YAML's
// {redacted: true} marker for the exact row comparison.
func checkRedactedSelectedValues(t *testing.T, summaries []map[string]interface{}, keys ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, k := range keys {
		want[k] = true
	}
	for _, s := range summaries {
		list, _ := s["summaries"].([]interface{})
		for _, item := range list {
			sm, _ := item.(map[string]interface{})
			key, _ := sm["key"].(string)
			if !want[key] {
				continue
			}
			counters, _ := sm["counters"].([]interface{})
			for _, ci := range counters {
				c, _ := ci.(map[string]interface{})
				sv, _ := c["selectedValue"].(map[string]interface{})
				str, isString := sv["string"].(string)
				if len(sv) != 1 || !isString {
					t.Errorf("%s: selectedValue %v is not a redacted string", key, c["selectedValue"])
					continue
				}
				if !assert.Regexp(t, `^\*{5}[0-9a-f]{5}$`, str, "%s: selectedValue is not redacted", key) {
					continue
				}
				c["selectedValue"] = map[string]interface{}{"redacted": true}
			}
		}
	}
}
