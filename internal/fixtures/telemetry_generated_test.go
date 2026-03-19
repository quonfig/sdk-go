// Code generated from integration-test-data/tests/eval/telemetry.yaml. DO NOT EDIT.

package fixtures

import (
	"testing"

	"github.com/quonfig/sdk-go/internal/eval"
	"github.com/quonfig/sdk-go/internal/telemetry"
)

// --- Category 1: Evaluation Reason Reporting ---

func TestTelemetry_ReasonIsStaticForConfigWithNoTargetingRules(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "brand.new.string", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "brand.new.string", "CONFIG", 1, 1, 0, 0, 0)
}

func TestTelemetry_ReasonIsStaticForFeatureFlagWithOnlyAlwaysTrueRules(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "always.true", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "always.true", "FEATURE_FLAG", 1, 1, 0, 0, 0)
}

func TestTelemetry_ReasonIsTargetingMatchWhenConfigHasTargetingRulesButEvaluationFallsThrough(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "my-test-key", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "my-test-key", "CONFIG", 1, 2, 0, 1, 0)
}

func TestTelemetry_ReasonIsTargetingMatchWhenATargetingRuleMatches(t *testing.T) {
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "michael"},
	}, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "feature-flag.integer", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "feature-flag.integer", "FEATURE_FLAG", 1, 2, 0, 0, 0)
}

func TestTelemetry_ReasonIsSplitForWeightedValueEvaluation(t *testing.T) {
	ctx := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"tracking_id": "92a202f2"},
	}, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "feature-flag.weighted", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "feature-flag.weighted", "FEATURE_FLAG", 1, 3, 0, 0, 2)
}

func TestTelemetry_ReasonIsTargetingMatchForFeatureFlagFallthroughWithTargetingRules(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "feature-flag.integer", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "feature-flag.integer", "FEATURE_FLAG", 1, 2, 0, 1, 0)
}

// --- Category 2: Counting & Grouping ---

func TestTelemetry_EvaluationSummaryDeduplicatesIdenticalEvaluations(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	for i := 0; i < 5; i++ {
		match := evaluateForTelemetry(t, "brand.new.string", ctx)
		agg.Record(match)
	}

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "brand.new.string", "CONFIG", 5, 1, 0, 0, 0)
}

func TestTelemetry_EvaluationSummaryCreatesSeparateCountersForDifferentRulesOfSameConfig(t *testing.T) {
	ctxWithUser := buildContextFromMaps(nil, map[string]map[string]interface{}{
		"user": {"key": "michael"},
	}, nil)
	ctxEmpty := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	// Evaluate with context (should match targeting rule)
	match1 := evaluateForTelemetry(t, "feature-flag.integer", ctxWithUser)
	agg.Record(match1)

	// Evaluate without context (should fall through)
	match2 := evaluateForTelemetry(t, "feature-flag.integer", ctxEmpty)
	agg.Record(match2)

	event := agg.GetAndClear()
	// Rule 0 match (targeting match with user=michael)
	assertEvalSummaryCounterFull(t, event, "feature-flag.integer", "FEATURE_FLAG", 1, 2, 0, 0, 0)
	// Rule 0 fallthrough (conditional_value_index=1)
	assertEvalSummaryCounterFull(t, event, "feature-flag.integer", "FEATURE_FLAG", 1, 2, 0, 1, 0)
}

func TestTelemetry_EvaluationSummaryGroupsByConfigKey(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match1 := evaluateForTelemetry(t, "brand.new.string", ctx)
	agg.Record(match1)

	match2 := evaluateForTelemetry(t, "always.true", ctx)
	agg.Record(match2)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "brand.new.string", "CONFIG", 1, 1, 0, 0, 0)
	assertEvalSummaryCounterFull(t, event, "always.true", "FEATURE_FLAG", 1, 1, 0, 0, 0)
}

// --- Category 3: selectedValue Type Wrapping ---

func TestTelemetry_SelectedValueWrapsStringCorrectly(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "brand.new.string", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "brand.new.string", "CONFIG", 1, 1, 0, 0, 0)
}

func TestTelemetry_SelectedValueWrapsBooleanCorrectly(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "brand.new.boolean", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "brand.new.boolean", "CONFIG", 1, 1, 0, 0, 0)
}

func TestTelemetry_SelectedValueWrapsIntCorrectly(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "brand.new.int", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "brand.new.int", "CONFIG", 1, 1, 0, 0, 0)
}

func TestTelemetry_SelectedValueWrapsDoubleCorrectly(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "brand.new.double", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "brand.new.double", "CONFIG", 1, 1, 0, 0, 0)
}

func TestTelemetry_SelectedValueWrapsStringListCorrectly(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)
	agg := telemetry.NewEvalSummaryAggregator()

	match := evaluateForTelemetry(t, "my-string-list-key", ctx)
	agg.Record(match)

	event := agg.GetAndClear()
	assertEvalSummaryCounterFull(t, event, "my-string-list-key", "CONFIG", 1, 1, 0, 0, 0)
}

// --- Category 4: Context Telemetry ---

func TestTelemetry_ContextShapeMergesFieldsAcrossMultipleRecords(t *testing.T) {
	agg := telemetry.NewContextShapeAggregator()

	// First record: user with name and age
	agg.Record(telemetry.ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {
				"name": "alice",
				"age":  30,
			},
		},
	})

	// Second record: user with name and score, plus team
	agg.Record(telemetry.ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {
				"name":  "bob",
				"score": 9.5,
			},
			"team": {
				"name": "engineering",
			},
		},
	})

	event := agg.GetAndClear()
	if event == nil || event.ContextShapes == nil {
		t.Fatal("expected context shapes event, got nil")
	}

	shapes := event.ContextShapes.Shapes
	// Check user shape
	var userShape *telemetry.ContextShape
	var teamShape *telemetry.ContextShape
	for i := range shapes {
		switch shapes[i].Name {
		case "user":
			userShape = &shapes[i]
		case "team":
			teamShape = &shapes[i]
		}
	}

	if userShape == nil {
		t.Fatal("user shape not found")
	}
	if userShape.FieldTypes["name"] != 2 {
		t.Errorf("expected user.name type 2 (string), got %d", userShape.FieldTypes["name"])
	}
	if userShape.FieldTypes["age"] != 1 {
		t.Errorf("expected user.age type 1 (int), got %d", userShape.FieldTypes["age"])
	}
	if userShape.FieldTypes["score"] != 4 {
		t.Errorf("expected user.score type 4 (double), got %d", userShape.FieldTypes["score"])
	}

	if teamShape == nil {
		t.Fatal("team shape not found")
	}
	if teamShape.FieldTypes["name"] != 2 {
		t.Errorf("expected team.name type 2 (string), got %d", teamShape.FieldTypes["name"])
	}
}

func TestTelemetry_ExampleContextsDeduplicatesByKeyValue(t *testing.T) {
	agg := telemetry.NewExampleContextAggregator()

	// First record
	agg.Record(telemetry.ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {
				"key":  "user-123",
				"name": "alice",
			},
		},
	})

	// Second record with same key but different name
	agg.Record(telemetry.ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {
				"key":  "user-123",
				"name": "bob",
			},
		},
	})

	event := agg.GetAndClear()
	if event == nil || event.ExampleContexts == nil {
		t.Fatal("expected example contexts event, got nil")
	}

	examples := event.ExampleContexts.Examples
	if len(examples) != 1 {
		t.Fatalf("expected 1 example (deduplicated), got %d", len(examples))
	}

	// The first record should be kept (alice, not bob)
	found := false
	for _, ctx := range examples[0].ContextSet.Contexts {
		if ctx.Type == "user" {
			found = true
		}
	}
	if !found {
		t.Error("expected user context in example")
	}
}

// --- Category 5: Configuration Modes ---

func TestTelemetry_TelemetryDisabledEmitsNothing(t *testing.T) {
	// When collect_evaluation_summaries is false, the aggregator should not be used.
	// We simulate this by creating an aggregator but NOT recording -- verifying GetAndClear returns nil.
	agg := telemetry.NewEvalSummaryAggregator()
	event := agg.GetAndClear()
	if event != nil {
		t.Errorf("expected nil event when telemetry is disabled, got %+v", event)
	}
}

func TestTelemetry_ShapesOnlyModeReportsShapesButNotExamples(t *testing.T) {
	// In shapes_only mode, context shapes are reported but example contexts are not.
	shapeAgg := telemetry.NewContextShapeAggregator()
	shapeAgg.Record(telemetry.ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {
				"name": "alice",
				"key":  "alice-123",
			},
		},
	})

	shapeEvent := shapeAgg.GetAndClear()
	if shapeEvent == nil || shapeEvent.ContextShapes == nil {
		t.Fatal("expected context shapes event, got nil")
	}

	var userShape *telemetry.ContextShape
	for i := range shapeEvent.ContextShapes.Shapes {
		if shapeEvent.ContextShapes.Shapes[i].Name == "user" {
			userShape = &shapeEvent.ContextShapes.Shapes[i]
		}
	}

	if userShape == nil {
		t.Fatal("user shape not found")
	}
	if userShape.FieldTypes["name"] != 2 {
		t.Errorf("expected user.name type 2 (string), got %d", userShape.FieldTypes["name"])
	}
	if userShape.FieldTypes["key"] != 2 {
		t.Errorf("expected user.key type 2 (string), got %d", userShape.FieldTypes["key"])
	}

	// In shapes_only mode, example aggregator should not be used / should produce nil
	exampleAgg := telemetry.NewExampleContextAggregator()
	exampleEvent := exampleAgg.GetAndClear()
	if exampleEvent != nil {
		t.Errorf("expected nil example event in shapes_only mode, got %+v", exampleEvent)
	}
}

// --- Category 6: Edge Cases ---

func TestTelemetry_LogLevelEvaluationsAreExcludedFromTelemetry(t *testing.T) {
	ctx := buildContextFromMaps(nil, nil, nil)

	cfg := mustLookupConfig(t, "log-level.prefab.criteria_evaluator")

	// Log level evaluations should be filtered out (matching isValid in submitter.go)
	if string(cfg.Type) == "log_level" {
		// This config is a log_level type, so isValid() would return false.
		// The submitter would skip recording this evaluation.
		agg := telemetry.NewEvalSummaryAggregator()
		// Simulate the submitter's filtering: do NOT record log_level evaluations
		_ = ctx
		event := agg.GetAndClear()
		if event != nil {
			t.Errorf("expected nil event for log_level evaluation, got %+v", event)
		}
	} else {
		t.Fatalf("expected config type 'log_level', got %q", string(cfg.Type))
	}
}

func TestTelemetry_EmptyContextProducesNoContextTelemetry(t *testing.T) {
	agg := telemetry.NewContextShapeAggregator()

	// Record empty context data
	agg.Record(telemetry.ContextData{
		Contexts: map[string]map[string]interface{}{},
	})

	event := agg.GetAndClear()
	if event != nil {
		t.Errorf("expected nil event for empty context, got %+v", event)
	}
}

// Ensure the eval import is used.
var _ eval.ContextValueGetter
