// Code generated from integration-test-data/tests/eval/post.yaml. DO NOT EDIT.
// Regenerate with:
//   cd integration-test-data/generators && npm run generate -- --target=go
// Source: integration-test-data/generators/src/targets/go.ts

package fixtures

import (
	"testing"
)

// reports context shape aggregation
func TestPost_ReportsContextShapeAggregation(t *testing.T) {
	agg := BuildAggregator(t, "context_shape", map[string]interface{}{"context_upload_mode": ":shape_only"})
	FeedAggregator(t, agg, "context_shape", map[string]interface{}{"user": map[string]interface{}{"name": "Michael", "age": 38, "human": true}, "role": map[string]interface{}{"name": "developer", "admin": false, "salary": 15.75, "permissions": []interface{}{"read", "write"}}}, nil)
	AssertAggregatorPost(t, agg, "context_shape", []interface{}{map[string]interface{}{"name": "user", "field_types": map[string]interface{}{"name": 2, "age": 1, "human": 5}}, map[string]interface{}{"name": "role", "field_types": map[string]interface{}{"name": 2, "admin": 5, "salary": 4, "permissions": 10}}}, "/api/v1/context-shapes")
}

// reports evaluation summary
func TestPost_ReportsEvaluationSummary(t *testing.T) {
	agg := BuildAggregator(t, "evaluation_summary", map[string]interface{}{})
	FeedAggregator(t, agg, "evaluation_summary", map[string]interface{}{"keys": []interface{}{"my-test-key", "feature-flag.integer", "my-string-list-key", "feature-flag.integer", "feature-flag.weighted"}}, map[string]map[string]interface{}{"user": {"tracking_id": "92a202f2"}})
	AssertAggregatorPost(t, agg, "evaluation_summary", []interface{}{map[string]interface{}{"key": "my-test-key", "type": "CONFIG", "value": "my-test-value", "value_type": "string", "count": 1, "reason": 2, "selected_value": map[string]interface{}{"string": "my-test-value"}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 1}}, map[string]interface{}{"key": "my-string-list-key", "type": "CONFIG", "value": []interface{}{"a", "b", "c"}, "value_type": "string_list", "count": 1, "reason": 1, "selected_value": map[string]interface{}{"stringList": []interface{}{"a", "b", "c"}}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0}}, map[string]interface{}{"key": "feature-flag.integer", "type": "FEATURE_FLAG", "value": 3, "value_type": "int", "count": 2, "reason": 2, "selected_value": map[string]interface{}{"int": 3}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 1}}, map[string]interface{}{"key": "feature-flag.weighted", "type": "FEATURE_FLAG", "value": 2, "value_type": "int", "count": 1, "reason": 3, "selected_value": map[string]interface{}{"int": 2}, "summary": map[string]interface{}{"config_row_index": 0, "conditional_value_index": 0, "weighted_value_index": 2}}}, "/api/v1/telemetry")
}

// reports example contexts
func TestPost_ReportsExampleContexts(t *testing.T) {
	agg := BuildAggregator(t, "example_contexts", map[string]interface{}{})
	FeedAggregator(t, agg, "example_contexts", map[string]interface{}{"user": map[string]interface{}{"name": "michael", "age": 38, "key": "michael:1234"}, "device": map[string]interface{}{"mobile": false}, "team": map[string]interface{}{"id": 3.5}}, nil)
	AssertAggregatorPost(t, agg, "example_contexts", map[string]interface{}{"user": map[string]interface{}{"name": "michael", "age": 38, "key": "michael:1234"}, "device": map[string]interface{}{"mobile": false}, "team": map[string]interface{}{"id": 3.5}}, "/api/v1/telemetry")
}

// example contexts without key are not reported
func TestPost_ExampleContextsWithoutKeyAreNotReported(t *testing.T) {
	agg := BuildAggregator(t, "example_contexts", map[string]interface{}{})
	FeedAggregator(t, agg, "example_contexts", map[string]interface{}{"user": map[string]interface{}{"name": "michael", "age": 38}, "device": map[string]interface{}{"mobile": false}, "team": map[string]interface{}{"id": 3.5}}, nil)
	AssertAggregatorPost(t, agg, "example_contexts", nil, "/api/v1/telemetry")
}
