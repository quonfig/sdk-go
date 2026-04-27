// Code generated from integration-test-data/tests/eval/telemetry.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// reason is STATIC for config with no targeting rules
func TestTelemetry_ReasonIsSTATICForConfigWithNoTargetingRules(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"brand.new.string"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "hello.world"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// reason is STATIC for feature flag with only ALWAYS_TRUE rules
func TestTelemetry_ReasonIsSTATICForFeatureFlagWithOnlyALWAYSTRUERules(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"always.true"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "always.true", "type": "FEATURE_FLAG", "value": true, "value_type": "bool", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"bool": true}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// reason is TARGETING_MATCH when config has targeting rules but evaluation falls through
func TestTelemetry_ReasonIsTARGETINGMATCHWhenConfigHasTargetingRulesButEvaluationFallsThrough(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"my-test-key"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "my-test-key", "type": "CONFIG", "value": "my-test-value", "value_type": "string", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"string": "my-test-value"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 1}}}, "/api/v1/telemetry")
}

// reason is TARGETING_MATCH when a targeting rule matches
func TestTelemetry_ReasonIsTARGETINGMATCHWhenATargetingRuleMatches(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"feature-flag.integer"}}, map[string]map[string]interface{}{"user": {"key": "michael"}})
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "feature-flag.integer", "type": "FEATURE_FLAG", "value": 5, "value_type": "int", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"int": 5}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// reason is SPLIT for weighted value evaluation
func TestTelemetry_ReasonIsSPLITForWeightedValueEvaluation(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"feature-flag.weighted"}}, map[string]map[string]interface{}{"user": {"tracking_id": "92a202f2"}})
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "feature-flag.weighted", "type": "FEATURE_FLAG", "value": 2, "value_type": "int", "count": 1, "reason": 3, "selected_value": map[string]interface{}{"int": 2}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0, "weighted_value_index": 2}}}, "/api/v1/telemetry")
}

// reason is TARGETING_MATCH for feature flag fallthrough with targeting rules
func TestTelemetry_ReasonIsTARGETINGMATCHForFeatureFlagFallthroughWithTargetingRules(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"feature-flag.integer"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "feature-flag.integer", "type": "FEATURE_FLAG", "value": 3, "value_type": "int", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"int": 3}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 1}}}, "/api/v1/telemetry")
}

// evaluation summary deduplicates identical evaluations
func TestTelemetry_EvaluationSummaryDeduplicatesIdenticalEvaluations(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"brand.new.string", "brand.new.string", "brand.new.string", "brand.new.string", "brand.new.string"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 5, "reason": 1, "selected_value": map[string]interface{}{"string": "hello.world"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// evaluation summary creates separate counters for different rules of same config
func TestTelemetry_EvaluationSummaryCreatesSeparateCountersForDifferentRulesOfSameConfig(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"feature-flag.integer"}, "keys_without_context": []interface{}{"feature-flag.integer"}}, map[string]map[string]interface{}{"user": {"key": "michael"}})
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "feature-flag.integer", "type": "FEATURE_FLAG", "value": 5, "value_type": "int", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"int": 5}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}, map[string]interface{}{"key": "feature-flag.integer", "type": "FEATURE_FLAG", "value": 3, "value_type": "int", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"int": 3}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 1}}}, "/api/v1/telemetry")
}

// evaluation summary groups by config key
func TestTelemetry_EvaluationSummaryGroupsByConfigKey(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"brand.new.string", "always.true"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "hello.world"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}, map[string]interface{}{"key": "always.true", "type": "FEATURE_FLAG", "value": true, "value_type": "bool", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"bool": true}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps string correctly
func TestTelemetry_SelectedValueWrapsStringCorrectly(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"brand.new.string"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "hello.world"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps boolean correctly
func TestTelemetry_SelectedValueWrapsBooleanCorrectly(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"brand.new.boolean"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.boolean", "type": "CONFIG", "value": false, "value_type": "bool", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"bool": false}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps int correctly
func TestTelemetry_SelectedValueWrapsIntCorrectly(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"brand.new.int"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.int", "type": "CONFIG", "value": 123, "value_type": "int", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"int": 123}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps double correctly
func TestTelemetry_SelectedValueWrapsDoubleCorrectly(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"brand.new.double"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "brand.new.double", "type": "CONFIG", "value": 123.99, "value_type": "double", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"double": 123.99}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// selectedValue wraps string list correctly
func TestTelemetry_SelectedValueWrapsStringListCorrectly(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"my-string-list-key"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "my-string-list-key", "type": "CONFIG", "value": []interface{}{"a", "b", "c"}, "value_type": "string_list", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"stringList": []interface{}{"a", "b", "c"}}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// context shape merges fields across multiple records
func TestTelemetry_ContextShapeMergesFieldsAcrossMultipleRecords(t *testing.T) {
	agg := BuildAggregator(t, "context_shape", map[string]interface{}{})
	FeedAggregator(t, agg, "context_shape", []interface{}{map[string]interface{}{"user": map[string]interface{}{"name": "alice", "age": 30}}, map[string]interface{}{"user": map[string]interface{}{"name": "bob", "score": 9.5}, "team": map[string]interface{}{"name": "engineering"}}}, nil)
	AssertAggregatorPost(t, agg, "context_shape", []interface{}{map[string]interface{}{"name": "user", "field_types": map[string]interface{}{"name": 2, "age": 1, "score": 4}}, map[string]interface{}{"name": "team", "field_types": map[string]interface{}{"name": 2}}}, "/api/v1/context-shapes")
}

// example contexts deduplicates by key value
func TestTelemetry_ExampleContextsDeduplicatesByKeyValue(t *testing.T) {
	agg := BuildAggregator(t, "example_contexts", map[string]interface{}{})
	FeedAggregator(t, agg, "example_contexts", []interface{}{map[string]interface{}{"user": map[string]interface{}{"key": "user-123", "name": "alice"}}, map[string]interface{}{"user": map[string]interface{}{"key": "user-123", "name": "bob"}}}, nil)
	AssertAggregatorPost(t, agg, "example_contexts", map[string]interface{}{"user": map[string]interface{}{"key": "user-123", "name": "alice"}}, "/api/v1/telemetry")
}

// telemetry disabled emits nothing
func TestTelemetry_TelemetryDisabledEmitsNothing(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{"collect_evaluation_summaries": false, "context_upload_mode": ":none"})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"brand.new.string"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", nil, "/api/v1/telemetry")
}

// shapes only mode reports shapes but not examples
func TestTelemetry_ShapesOnlyModeReportsShapesButNotExamples(t *testing.T) {
	agg := BuildAggregator(t, "context_shape", map[string]interface{}{"context_upload_mode": ":shape_only"})
	FeedAggregator(t, agg, "context_shape", map[string]interface{}{"user": map[string]interface{}{"name": "alice", "key": "alice-123"}}, nil)
	AssertAggregatorPost(t, agg, "context_shape", []interface{}{map[string]interface{}{"name": "user", "field_types": map[string]interface{}{"name": 2, "key": 2}}}, "/api/v1/context-shapes")
}

// log level evaluations are excluded from telemetry
func TestTelemetry_LogLevelEvaluationsAreExcludedFromTelemetry(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"log-level.prefab.criteria_evaluator"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", nil, "/api/v1/telemetry")
}

// empty context produces no context telemetry
func TestTelemetry_EmptyContextProducesNoContextTelemetry(t *testing.T) {
	agg := BuildAggregator(t, "context_shape", map[string]interface{}{})
	FeedAggregator(t, agg, "context_shape", map[string]interface{}{}, nil)
	AssertAggregatorPost(t, agg, "context_shape", nil, "/api/v1/context-shapes")
}

// confidential plain string is redacted in selectedValue
func TestTelemetry_ConfidentialPlainStringIsRedactedInSelectedValue(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"confidential.new.string"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "confidential.new.string", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "*****18aa7"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}

// confidential encrypted string is redacted using ciphertext hash
func TestTelemetry_ConfidentialEncryptedStringIsRedactedUsingCiphertextHash(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"a.secret.config"}}, nil)
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "a.secret.config", "type": "CONFIG", "value": "hello.world", "value_type": "string", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"string": "*****936c9"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}}, "/api/v1/telemetry")
}
