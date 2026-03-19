package telemetry

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// EvalMatch holds the data needed to record an evaluation for telemetry.
// This is the telemetry package's view of an evaluation result.
type EvalMatch struct {
	ConfigID           string
	ConfigKey          string
	ConfigType         string
	RuleIndex          int
	WeightedValueIndex int
	SelectedValue      interface{}
	Reason             int
}

// EvalSummaryAggregator accumulates evaluation counts grouped by
// config key + rule index + weighted value index + selected value.
type EvalSummaryAggregator struct {
	mu        sync.Mutex
	data      map[string]EvalMatch
	counts    map[string]int64
	dataStart int64
}

// NewEvalSummaryAggregator creates a new aggregator.
func NewEvalSummaryAggregator() *EvalSummaryAggregator {
	return &EvalSummaryAggregator{
		data:   make(map[string]EvalMatch),
		counts: make(map[string]int64),
	}
}

// Record adds an evaluation to the aggregator.
func (a *EvalSummaryAggregator) Record(match EvalMatch) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.dataStart == 0 {
		a.dataStart = time.Now().UnixMilli()
	}

	key := fmt.Sprintf("%s-%d-%d-%v", match.ConfigID, match.RuleIndex, match.WeightedValueIndex, match.SelectedValue)

	if _, ok := a.data[key]; !ok {
		a.data[key] = match
		a.counts[key] = 1
	} else {
		a.counts[key]++
	}
}

// GetAndClear returns the current summaries and resets state. Returns nil if empty.
func (a *EvalSummaryAggregator) GetAndClear() *TelemetryEvent {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.data) == 0 {
		return nil
	}

	// Group counters by config key + config type
	type counterKey struct {
		key        string
		configType string
	}
	grouped := make(map[counterKey][]EvalCounter)

	for groupingKey, match := range a.data {
		selectedValueJSON, _ := json.Marshal(marshalSelectedValue(match.SelectedValue))

		counter := EvalCounter{
			ConfigID:              match.ConfigID,
			ConditionalValueIndex: match.RuleIndex,
			ConfigRowIndex:        0,
			WeightedValueIndex:    match.WeightedValueIndex,
			SelectedValue:         selectedValueJSON,
			Count:                 a.counts[groupingKey],
			Reason:                match.Reason,
		}

		ck := counterKey{key: match.ConfigKey, configType: match.ConfigType}
		grouped[ck] = append(grouped[ck], counter)
	}

	summaries := make([]EvalSummary, 0, len(grouped))
	for ck, counters := range grouped {
		summaries = append(summaries, EvalSummary{
			Key:      ck.key,
			Type:     ck.configType,
			Counters: counters,
		})
	}

	event := &TelemetryEvent{
		Summaries: &EvalSummaries{
			Start:     a.dataStart,
			End:       time.Now().UnixMilli(),
			Summaries: summaries,
		},
	}

	// Reset
	a.data = make(map[string]EvalMatch)
	a.counts = make(map[string]int64)
	a.dataStart = 0

	return event
}

// marshalSelectedValue converts a value to the JSON-friendly format used in the telemetry payload.
func marshalSelectedValue(v interface{}) interface{} {
	switch val := v.(type) {
	case bool:
		return map[string]interface{}{"bool": val}
	case int64:
		return map[string]interface{}{"int": val}
	case float64:
		return map[string]interface{}{"double": val}
	case string:
		return map[string]interface{}{"string": val}
	case []string:
		return map[string]interface{}{"stringList": val}
	default:
		return map[string]interface{}{"string": fmt.Sprintf("%v", val)}
	}
}
