package telemetry

import (
	"encoding/json"
	"testing"
)

func TestEvalSummaryAggregator_Empty(t *testing.T) {
	agg := NewEvalSummaryAggregator()
	event := agg.GetAndClear()
	if event != nil {
		t.Fatal("expected nil event for empty aggregator")
	}
}

func TestEvalSummaryAggregator_SingleEvaluation(t *testing.T) {
	agg := NewEvalSummaryAggregator()

	agg.Record(EvalMatch{
		ConfigID:           "cfg-123",
		ConfigKey:          "feature.flag",
		ConfigType:         "feature_flag",
		RuleIndex:          0,
		WeightedValueIndex: 0,
		SelectedValue:      true,
		Reason:             2, // TARGETING_MATCH
	})

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.Summaries == nil {
		t.Fatal("expected summaries")
	}
	if len(event.Summaries.Summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(event.Summaries.Summaries))
	}

	summary := event.Summaries.Summaries[0]
	if summary.Key != "feature.flag" {
		t.Errorf("expected key 'feature.flag', got %q", summary.Key)
	}
	if summary.Type != "feature_flag" {
		t.Errorf("expected type 'feature_flag', got %q", summary.Type)
	}
	if len(summary.Counters) != 1 {
		t.Fatalf("expected 1 counter, got %d", len(summary.Counters))
	}
	if summary.Counters[0].Count != 1 {
		t.Errorf("expected count 1, got %d", summary.Counters[0].Count)
	}
	if summary.Counters[0].Reason != 2 {
		t.Errorf("expected reason 2, got %d", summary.Counters[0].Reason)
	}

	// Verify selectedValue is marshaled correctly
	var sv map[string]interface{}
	if err := json.Unmarshal(summary.Counters[0].SelectedValue, &sv); err != nil {
		t.Fatalf("failed to unmarshal selectedValue: %v", err)
	}
	if sv["bool"] != true {
		t.Errorf("expected selectedValue {bool: true}, got %v", sv)
	}
}

func TestEvalSummaryAggregator_CountsDuplicates(t *testing.T) {
	agg := NewEvalSummaryAggregator()

	match := EvalMatch{
		ConfigID:      "cfg-123",
		ConfigKey:     "feature.flag",
		ConfigType:    "feature_flag",
		SelectedValue: true,
		Reason:        1,
	}

	for i := 0; i < 100; i++ {
		agg.Record(match)
	}

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if event.Summaries.Summaries[0].Counters[0].Count != 100 {
		t.Errorf("expected count 100, got %d", event.Summaries.Summaries[0].Counters[0].Count)
	}
}

func TestEvalSummaryAggregator_ClearsAfterGet(t *testing.T) {
	agg := NewEvalSummaryAggregator()

	agg.Record(EvalMatch{
		ConfigID:      "cfg-123",
		ConfigKey:     "feature.flag",
		ConfigType:    "feature_flag",
		SelectedValue: true,
	})

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}

	event2 := agg.GetAndClear()
	if event2 != nil {
		t.Fatal("expected nil after clear")
	}
}

func TestEvalSummaryAggregator_GroupsByRuleAndValue(t *testing.T) {
	agg := NewEvalSummaryAggregator()

	// Same config, different rule indices -> different counters
	agg.Record(EvalMatch{
		ConfigID:      "cfg-1",
		ConfigKey:     "my.flag",
		ConfigType:    "feature_flag",
		RuleIndex:     0,
		SelectedValue: true,
	})
	agg.Record(EvalMatch{
		ConfigID:      "cfg-1",
		ConfigKey:     "my.flag",
		ConfigType:    "feature_flag",
		RuleIndex:     1,
		SelectedValue: false,
	})

	event := agg.GetAndClear()
	if event == nil {
		t.Fatal("expected non-nil event")
	}
	if len(event.Summaries.Summaries) != 1 {
		t.Fatalf("expected 1 summary (grouped by key), got %d", len(event.Summaries.Summaries))
	}
	if len(event.Summaries.Summaries[0].Counters) != 2 {
		t.Errorf("expected 2 counters (different rules), got %d", len(event.Summaries.Summaries[0].Counters))
	}
}

func TestEvalSummaryAggregator_TimeWindow(t *testing.T) {
	agg := NewEvalSummaryAggregator()

	agg.Record(EvalMatch{
		ConfigID:      "cfg-1",
		ConfigKey:     "flag",
		ConfigType:    "feature_flag",
		SelectedValue: true,
	})

	event := agg.GetAndClear()
	if event.Summaries.Start == 0 {
		t.Error("expected non-zero start")
	}
	if event.Summaries.End == 0 {
		t.Error("expected non-zero end")
	}
	if event.Summaries.End < event.Summaries.Start {
		t.Error("expected end >= start")
	}
}
