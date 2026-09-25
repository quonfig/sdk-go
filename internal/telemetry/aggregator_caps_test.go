package telemetry

import (
	"fmt"
	"testing"
)

func TestEvalSummaryAggregator_CapKeepsCountingExistingKeys(t *testing.T) {
	a := NewEvalSummaryAggregatorWithCap(2)
	for i := 0; i < 4; i++ {
		a.Record(EvalMatch{ConfigID: fmt.Sprintf("c%d", i), ConfigKey: fmt.Sprintf("k%d", i), ConfigType: "config", SelectedValue: "v"})
	}
	a.Record(EvalMatch{ConfigID: "c0", ConfigKey: "k0", ConfigType: "config", SelectedValue: "v"})
	ev := a.GetAndClear()
	counts := map[string]int64{}
	for _, s := range ev.Summaries.Summaries {
		for _, c := range s.Counters {
			counts[s.Key] += c.Count
		}
	}
	if len(counts) != 2 || counts["k0"] != 2 {
		t.Fatalf("counts = %v, want k0=2 and one other key", counts)
	}
	if NewEvalSummaryAggregator().maxKeys != DefaultMaxEvaluationSummaries {
		t.Fatal("default eval summary cap")
	}
}

func TestContextShapeAggregator_CapsFieldPairs(t *testing.T) {
	a := NewContextShapeAggregatorWithCap(3)
	a.Record(ContextData{Contexts: map[string]map[string]interface{}{"user": {"a": 1, "b": 2}}})
	a.Record(ContextData{Contexts: map[string]map[string]interface{}{"team": {"c": "x", "d": "y"}}})
	a.Record(ContextData{Contexts: map[string]map[string]interface{}{"user": {"a": "now-a-string"}}})
	ev := a.GetAndClear()
	n := 0
	for _, s := range ev.ContextShapes.Shapes {
		n += len(s.FieldTypes)
		if s.Name == "user" && s.FieldTypes["a"] != FieldTypeString {
			t.Fatal("existing field did not update at the cap")
		}
	}
	if n != 3 {
		t.Fatalf("fields = %d, want 3", n)
	}
	if NewContextShapeAggregator().maxFields != DefaultMaxContextShapeFields {
		t.Fatal("default shape cap")
	}
}

func TestExampleContextAggregator_Cap(t *testing.T) {
	a := NewExampleContextAggregatorWithCap(2)
	for i := 0; i < 5; i++ {
		a.Record(ContextData{Contexts: map[string]map[string]interface{}{"user": {"key": fmt.Sprintf("u%d", i)}}})
	}
	if n := len(a.GetAndClear().ExampleContexts.Examples); n != 2 {
		t.Fatalf("examples = %d, want 2", n)
	}
	if NewExampleContextAggregator().maxExamples != DefaultMaxExampleContexts {
		t.Fatal("default example cap")
	}
}
