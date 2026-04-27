package fixtures

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"testing"
	"time"

	quonfig "github.com/quonfig/sdk-go"
	"github.com/quonfig/sdk-go/internal/eval"
	"github.com/quonfig/sdk-go/internal/resolver"
	"github.com/quonfig/sdk-go/internal/telemetry"
)

// aggregatorHandle is the opaque container returned by BuildAggregator.
// It holds whichever real telemetry aggregator matches the kind, plus any
// client-side overrides that affect what (if anything) is reported.
type aggregatorHandle struct {
	kind                 string
	overrides            map[string]interface{}
	contextShape         *telemetry.ContextShapeAggregator
	evalSummary          *telemetry.EvalSummaryAggregator
	exampleContext       *telemetry.ExampleContextAggregator
	collectEvalSummaries bool
	contextUploadMode    string // ":none", ":shape_only", ":periodic_example" (default)
}

// BuildAggregator constructs the aggregator type that matches kind, honoring the
// supplied client-side overrides. It returns an opaque handle that can be passed
// to FeedAggregator and AssertAggregatorPost.
func BuildAggregator(t *testing.T, kind string, overrides map[string]interface{}) *aggregatorHandle {
	t.Helper()

	h := &aggregatorHandle{
		kind:                 kind,
		overrides:            overrides,
		collectEvalSummaries: true,
		contextUploadMode:    ":periodic_example",
	}

	if v, ok := overrides["collect_evaluation_summaries"]; ok {
		if b, ok := v.(bool); ok {
			h.collectEvalSummaries = b
		}
	}
	if v, ok := overrides["context_upload_mode"]; ok {
		if s, ok := v.(string); ok {
			h.contextUploadMode = s
		}
	}

	switch kind {
	case "context_shape":
		h.contextShape = telemetry.NewContextShapeAggregator()
	case "evaluation_summary":
		h.evalSummary = telemetry.NewEvalSummaryAggregator()
	case "example_contexts":
		h.exampleContext = telemetry.NewExampleContextAggregator()
	default:
		t.Fatalf("BuildAggregator: unknown kind %q", kind)
	}
	return h
}

// FeedAggregator drives the aggregator with the supplied data + context.
//
//   - For context_shape and example_contexts, data is either a map describing a
//     single context set, or a []interface{} of such maps for multi-record fixtures.
//   - For evaluation_summary, data is a map with a "keys" entry (and optionally
//     "keys_without_context") listing config keys to evaluate against ctx.
//
// The third argument is a map[string]map[string]interface{} of named contexts
// (name -> property name -> value) to use for evaluation/recording. nil means no
// context.
func FeedAggregator(t *testing.T, h *aggregatorHandle, kind string, data interface{}, ctx interface{}) {
	t.Helper()
	if h.kind != kind {
		t.Fatalf("FeedAggregator: kind mismatch (handle=%q, call=%q)", h.kind, kind)
	}

	switch kind {
	case "context_shape":
		if !shouldRecordContextShape(h) {
			return
		}
		feedContextShape(h.contextShape, data)
	case "example_contexts":
		if !shouldRecordExampleContext(h) {
			return
		}
		feedExampleContexts(h.exampleContext, data)
	case "evaluation_summary":
		if !h.collectEvalSummaries {
			return
		}
		feedEvalSummary(t, h.evalSummary, data, ctx)
	default:
		t.Fatalf("FeedAggregator: unknown kind %q", kind)
	}
}

// AssertAggregatorPost flushes the aggregator (GetAndClear) and asserts that the
// post payload for the given endpoint matches the expected shape from the YAML.
// expected may be nil to mean "nothing should be reported".
func AssertAggregatorPost(t *testing.T, h *aggregatorHandle, kind string, expected interface{}, endpoint string) {
	t.Helper()
	if h.kind != kind {
		t.Fatalf("AssertAggregatorPost: kind mismatch (handle=%q, call=%q)", h.kind, kind)
	}

	switch kind {
	case "context_shape":
		assertContextShapePost(t, h, expected, endpoint)
	case "evaluation_summary":
		assertEvalSummaryPost(t, h, expected, endpoint)
	case "example_contexts":
		assertExampleContextsPost(t, h, expected, endpoint)
	default:
		t.Fatalf("AssertAggregatorPost: unknown kind %q", kind)
	}
}

// shouldRecordContextShape mirrors the SDK's runtime gating: shapes are
// reported in :shape_only and :periodic_example modes, but not :none.
func shouldRecordContextShape(h *aggregatorHandle) bool {
	return h.contextUploadMode != ":none" && h.contextUploadMode != "none"
}

// shouldRecordExampleContext mirrors the SDK's runtime gating: example
// contexts only flow when the mode is :periodic_example.
func shouldRecordExampleContext(h *aggregatorHandle) bool {
	return h.contextUploadMode == ":periodic_example" || h.contextUploadMode == "periodic_example" || h.contextUploadMode == ""
}

// --- context_shape feeding -------------------------------------------------

func feedContextShape(agg *telemetry.ContextShapeAggregator, data interface{}) {
	for _, ctxMap := range expandContextRecords(data) {
		// Filter empty contexts; an empty map should produce no telemetry.
		if len(ctxMap) == 0 {
			continue
		}
		agg.Record(telemetry.ContextData{Contexts: ctxMap})
	}
}

// --- example_contexts feeding ----------------------------------------------

func feedExampleContexts(agg *telemetry.ExampleContextAggregator, data interface{}) {
	for _, ctxMap := range expandContextRecords(data) {
		if len(ctxMap) == 0 {
			continue
		}
		// Per the YAML spec: example contexts without a "key" property
		// anywhere in the set are not reported.
		if !contextSetHasKey(ctxMap) {
			continue
		}
		agg.Record(telemetry.ContextData{Contexts: ctxMap})
	}
}

func contextSetHasKey(ctxMap map[string]map[string]interface{}) bool {
	for _, props := range ctxMap {
		if _, ok := props["key"]; ok {
			return true
		}
	}
	return false
}

// expandContextRecords normalizes data into a slice of context maps. The data
// is either a single context map (already keyed by context name) or a
// []interface{} of such maps.
func expandContextRecords(data interface{}) []map[string]map[string]interface{} {
	if data == nil {
		return nil
	}
	switch v := data.(type) {
	case []interface{}:
		out := make([]map[string]map[string]interface{}, 0, len(v))
		for _, item := range v {
			if m := toContextMap(item); m != nil {
				out = append(out, m)
			}
		}
		return out
	case map[string]interface{}:
		if m := toContextMap(v); m != nil {
			return []map[string]map[string]interface{}{m}
		}
	}
	return nil
}

func toContextMap(v interface{}) map[string]map[string]interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	out := make(map[string]map[string]interface{}, len(m))
	for name, props := range m {
		if pm, ok := props.(map[string]interface{}); ok {
			// Copy to ensure stability and the right shape.
			cp := make(map[string]interface{}, len(pm))
			for k, val := range pm {
				cp[k] = val
			}
			out[name] = cp
		}
	}
	return out
}

// --- evaluation_summary feeding --------------------------------------------

func feedEvalSummary(t *testing.T, agg *telemetry.EvalSummaryAggregator, data interface{}, ctx interface{}) {
	t.Helper()

	dataMap, _ := data.(map[string]interface{})
	if dataMap == nil {
		t.Fatalf("FeedAggregator(evaluation_summary): expected map data, got %T", data)
	}

	ctxMap, _ := ctx.(map[string]map[string]interface{})

	if keys := dataMap["keys"]; keys != nil {
		evalKeysAndRecord(t, agg, keys, ctxMap)
	}
	if keys := dataMap["keys_without_context"]; keys != nil {
		evalKeysAndRecord(t, agg, keys, nil)
	}
}

func evalKeysAndRecord(t *testing.T, agg *telemetry.EvalSummaryAggregator, keys interface{}, ctxMap map[string]map[string]interface{}) {
	t.Helper()

	list, ok := keys.([]interface{})
	if !ok {
		t.Fatalf("FeedAggregator(evaluation_summary): expected keys list, got %T", keys)
	}

	contextValueGetter := buildContextFromMaps(nil, ctxMap, nil)

	for _, k := range list {
		key, ok := k.(string)
		if !ok {
			t.Fatalf("FeedAggregator(evaluation_summary): non-string key %T %v", k, k)
		}

		// Log-level evaluations are excluded from telemetry.
		cfg, ok := configStore.GetConfig(key)
		if !ok {
			t.Fatalf("FeedAggregator(evaluation_summary): config not found %q", key)
		}
		if isLogLevelConfig(cfg) {
			continue
		}

		match := evaluator.EvaluateConfig(cfg, "Production", contextValueGetter)

		var selectedValue interface{}
		var reportableValue *string
		reason := 4 // DEFAULT
		if match.IsMatch && match.Value != nil {
			// Compute the redacted reportable form BEFORE resolving, since
			// the resolver replaces the ciphertext with the decrypted
			// plaintext for decryptWith values.
			reportableValue = resolver.ReportableValueFor(match.Value)

			resolved, err := testResolver.Resolve(match.Value, cfg, "Production", contextValueGetter)
			if err != nil {
				t.Fatalf("evaluation_summary resolve failed for %s: %v", key, err)
			}
			selectedValue = resolved.Value
			switch {
			case match.WeightedValueIndex > 0:
				reason = 3 // SPLIT
			case !hasTargetingRules(cfg):
				reason = 1 // STATIC
			default:
				reason = 2 // TARGETING_MATCH
			}
		}

		agg.Record(telemetry.EvalMatch{
			ConfigID:           cfg.ID,
			ConfigKey:          cfg.Key,
			ConfigType:         configTypeToTelemetryType(cfg.Type),
			RuleIndex:          match.RuleIndex,
			WeightedValueIndex: match.WeightedValueIndex,
			SelectedValue:      selectedValue,
			ReportableValue:    reportableValue,
			Reason:             reason,
		})
	}
}

// --- context_shape assertion -----------------------------------------------

func assertContextShapePost(t *testing.T, h *aggregatorHandle, expected interface{}, endpoint string) {
	t.Helper()
	if endpoint != "/api/v1/context-shapes" {
		t.Fatalf("expected endpoint /api/v1/context-shapes, got %q", endpoint)
	}

	event := h.contextShape.GetAndClear()

	if expected == nil {
		if event != nil {
			t.Errorf("expected no context-shape post, got %+v", event)
		}
		return
	}

	if event == nil || event.ContextShapes == nil {
		t.Fatalf("expected context-shape post, got nil")
	}

	expectedList, ok := expected.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{} for context_shape expected_data, got %T", expected)
	}

	got := make([]map[string]interface{}, 0, len(event.ContextShapes.Shapes))
	for _, shape := range event.ContextShapes.Shapes {
		ft := make(map[string]interface{}, len(shape.FieldTypes))
		for k, v := range shape.FieldTypes {
			ft[k] = v
		}
		got = append(got, map[string]interface{}{
			"name":        shape.Name,
			"field_types": ft,
		})
	}

	want := make([]map[string]interface{}, 0, len(expectedList))
	for _, item := range expectedList {
		m, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("expected entry should be map, got %T", item)
		}
		want = append(want, m)
	}

	sortShapeList(got)
	sortShapeList(want)

	if !shapesEqual(got, want) {
		t.Errorf("context_shape mismatch:\n  got:  %s\n  want: %s", mustJSON(got), mustJSON(want))
	}
}

func sortShapeList(list []map[string]interface{}) {
	sort.SliceStable(list, func(i, j int) bool {
		ni, _ := list[i]["name"].(string)
		nj, _ := list[j]["name"].(string)
		return ni < nj
	})
}

func shapesEqual(a, b []map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i]["name"] != b[i]["name"] {
			return false
		}
		if !fieldTypesEqual(a[i]["field_types"], b[i]["field_types"]) {
			return false
		}
	}
	return true
}

func fieldTypesEqual(a, b interface{}) bool {
	am, _ := a.(map[string]interface{})
	bm, _ := b.(map[string]interface{})
	if len(am) != len(bm) {
		return false
	}
	for k, v := range am {
		bv, ok := bm[k]
		if !ok {
			return false
		}
		if intish(v) != intish(bv) {
			return false
		}
	}
	return true
}

// intish converts numeric-like interface{} values to int64 for comparison.
func intish(v interface{}) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}

// --- example_contexts assertion --------------------------------------------

func assertExampleContextsPost(t *testing.T, h *aggregatorHandle, expected interface{}, endpoint string) {
	t.Helper()
	if endpoint != "/api/v1/telemetry" {
		t.Fatalf("expected endpoint /api/v1/telemetry, got %q", endpoint)
	}

	event := h.exampleContext.GetAndClear()

	if expected == nil {
		if event != nil {
			t.Errorf("expected no example-contexts post, got %+v", event)
		}
		return
	}

	if event == nil || event.ExampleContexts == nil || len(event.ExampleContexts.Examples) == 0 {
		t.Fatalf("expected example-contexts post, got nil/empty")
	}

	wantMap, ok := expected.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{} for example_contexts expected_data, got %T", expected)
	}

	// Take the first example for comparison; YAML expects a single match.
	example := event.ExampleContexts.Examples[0]
	gotMap := exampleContextToMap(example)

	if !exampleContextsEqual(gotMap, wantMap) {
		t.Errorf("example_contexts mismatch:\n  got:  %s\n  want: %s", mustJSON(gotMap), mustJSON(wantMap))
	}
}

func exampleContextToMap(ex telemetry.ExampleContext) map[string]interface{} {
	out := make(map[string]interface{}, len(ex.ContextSet.Contexts))
	for _, ctx := range ex.ContextSet.Contexts {
		props := make(map[string]interface{}, len(ctx.Values))
		for k, v := range ctx.Values {
			props[k] = unwrapTypedContextValue(v)
		}
		out[ctx.Type] = props
	}
	return out
}

// unwrapTypedContextValue extracts the raw value from a TypedContextValue by
// round-tripping through JSON. The serialized form looks like
// {"int": N} / {"string": "x"} / {"bool": b} / {"double": d}.
func unwrapTypedContextValue(v telemetry.TypedContextValue) interface{} {
	raw, err := v.MarshalJSON()
	if err != nil {
		return nil
	}
	var wrapper map[string]interface{}
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil
	}
	for _, val := range wrapper {
		return val
	}
	return nil
}

func exampleContextsEqual(got, want map[string]interface{}) bool {
	if len(got) != len(want) {
		return false
	}
	for name, wantProps := range want {
		gotProps, ok := got[name].(map[string]interface{})
		if !ok {
			return false
		}
		wp, _ := wantProps.(map[string]interface{})
		if len(gotProps) != len(wp) {
			return false
		}
		for k, wv := range wp {
			gv, ok := gotProps[k]
			if !ok {
				return false
			}
			if !looseEqual(gv, wv) {
				return false
			}
		}
	}
	return true
}

// looseEqual compares two values across numeric kinds and JSON-decoded forms.
func looseEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	switch av := a.(type) {
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case int, int32, int64, float32, float64, json.Number:
		af, aOK := toFloat(a)
		bf, bOK := toFloat(b)
		return aOK && bOK && af == bf
	}
	return reflect.DeepEqual(a, b)
}

func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

// --- evaluation_summary assertion ------------------------------------------

func assertEvalSummaryPost(t *testing.T, h *aggregatorHandle, expected interface{}, endpoint string) {
	t.Helper()
	if endpoint != "/api/v1/telemetry" {
		t.Fatalf("expected endpoint /api/v1/telemetry, got %q", endpoint)
	}

	event := h.evalSummary.GetAndClear()

	if expected == nil {
		if event != nil {
			t.Errorf("expected no telemetry post, got %+v", event)
		}
		return
	}

	if event == nil || event.Summaries == nil {
		t.Fatalf("expected eval-summary post, got nil")
	}

	expectedList, ok := expected.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{} for evaluation_summary expected_data, got %T", expected)
	}

	got := flattenEvalSummaries(event.Summaries.Summaries)
	want := normalizeEvalSummaryExpected(expectedList)

	if !evalSummaryListsEqual(got, want) {
		t.Errorf("evaluation_summary mismatch:\n  got:  %s\n  want: %s", mustJSON(got), mustJSON(want))
	}
}

// flattenEvalSummaries expands [Summary{Counters}] into the flat row format
// used by the YAML expected_data.
func flattenEvalSummaries(summaries []telemetry.EvalSummary) []map[string]interface{} {
	out := make([]map[string]interface{}, 0)
	for _, s := range summaries {
		for _, c := range s.Counters {
			row := map[string]interface{}{
				"key":    s.Key,
				"type":   s.Type,
				"count":  c.Count,
				"reason": c.Reason,
				"summary": map[string]interface{}{
					"config_row_index":        c.ConfigRowIndex,
					"conditional_value_index": c.ConditionalValueIndex,
				},
			}
			if c.WeightedValueIndex != 0 {
				row["summary"].(map[string]interface{})["weighted_value_index"] = c.WeightedValueIndex
			}
			// Selected value: the JSON-encoded wrapper {"<type>": value}.
			var wrapper map[string]interface{}
			if len(c.SelectedValue) > 0 {
				_ = json.Unmarshal(c.SelectedValue, &wrapper)
			}
			row["selected_value"] = wrapper
			if wrapper != nil {
				for k, v := range wrapper {
					row["value_type"] = wrapperKeyToValueType(k)
					row["value"] = v
					break
				}
			}
			out = append(out, row)
		}
	}
	return out
}

// normalizeEvalSummaryExpected walks the expected list and ensures each row
// has both "value" and "selected_value" set when at least one of them is.
func normalizeEvalSummaryExpected(list []interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		// Ensure summary defaults exist.
		if _, ok := m["summary"]; !ok {
			m["summary"] = map[string]interface{}{}
		}
		// Synthesize selected_value from value+value_type if not already set.
		if _, hasSV := m["selected_value"]; !hasSV {
			vt, hasVT := m["value_type"].(string)
			val, hasVal := m["value"]
			if hasVT && hasVal {
				wrapperKey := valueTypeToWrapperKey(vt)
				m["selected_value"] = map[string]interface{}{wrapperKey: val}
			}
		}
		out = append(out, m)
	}
	return out
}

func valueTypeToWrapperKey(vt string) string {
	switch vt {
	case "string", "log_level", "duration":
		return "string"
	case "int":
		return "int"
	case "double":
		return "double"
	case "bool":
		return "bool"
	case "string_list":
		return "stringList"
	}
	return vt
}

// wrapperKeyToValueType converts a JSON wrapper key (e.g. "stringList") to the
// snake_case value_type token used in the YAML expected_data ("string_list").
func wrapperKeyToValueType(k string) string {
	switch k {
	case "stringList":
		return "string_list"
	}
	return k
}

func evalSummaryListsEqual(got, want []map[string]interface{}) bool {
	if len(got) != len(want) {
		return false
	}
	gotKey := func(r map[string]interface{}) string {
		k, _ := r["key"].(string)
		ty, _ := r["type"].(string)
		return fmt.Sprintf("%s|%s|%v|%v|%v", k, ty,
			r["count"], r["reason"],
			summaryKey(r["summary"]))
	}
	gotIdx := make(map[string]map[string]interface{}, len(got))
	for _, r := range got {
		gotIdx[gotKey(r)] = r
	}
	for _, w := range want {
		matchedKey := gotKey(w)
		gv, ok := gotIdx[matchedKey]
		if !ok {
			return false
		}
		if !evalSummaryRowEqual(gv, w) {
			return false
		}
		delete(gotIdx, matchedKey)
	}
	return len(gotIdx) == 0
}

func summaryKey(v interface{}) string {
	m, _ := v.(map[string]interface{})
	cri := intish(m["config_row_index"])
	cvi := intish(m["conditional_value_index"])
	wvi := intish(m["weighted_value_index"])
	return fmt.Sprintf("%d/%d/%d", cri, cvi, wvi)
}

func evalSummaryRowEqual(got, want map[string]interface{}) bool {
	// key + type
	if got["key"] != want["key"] {
		return false
	}
	if got["type"] != want["type"] {
		return false
	}
	if intish(got["count"]) != intish(want["count"]) {
		return false
	}
	if intish(got["reason"]) != intish(want["reason"]) {
		return false
	}
	// summary fields
	gs, _ := got["summary"].(map[string]interface{})
	ws, _ := want["summary"].(map[string]interface{})
	if intish(gs["config_row_index"]) != intish(ws["config_row_index"]) {
		return false
	}
	if intish(gs["conditional_value_index"]) != intish(ws["conditional_value_index"]) {
		return false
	}
	if intish(gs["weighted_value_index"]) != intish(ws["weighted_value_index"]) {
		return false
	}
	// value match (number-tolerant), only when expected sets "value".
	// Skip when the wire selected_value is a confidential redaction
	// ("*****<5-hex>"): in that case `value` in the YAML is the un-redacted
	// documentation form, while `selected_value` is the authoritative wire
	// payload.
	if wv, has := want["value"]; has {
		if !isRedactedSelectedValue(got["selected_value"]) {
			if !looseEqual(got["value"], wv) {
				return false
			}
		}
	}
	// value_type match
	if wt, has := want["value_type"]; has {
		if got["value_type"] != wt {
			return false
		}
	}
	// selected_value match (when present in want)
	if wsv, has := want["selected_value"]; has {
		if !selectedValueEqual(got["selected_value"], wsv) {
			return false
		}
	}
	return true
}

// isRedactedSelectedValue reports whether the wire selected_value is a
// confidential redaction (starts with "*****"). Used to skip the `value`
// match in evalSummaryRowEqual since the YAML's documentary `value` field
// holds the un-redacted form while `selected_value` holds the wire form.
func isRedactedSelectedValue(sv interface{}) bool {
	m, _ := sv.(map[string]interface{})
	if m == nil {
		return false
	}
	s, _ := m["string"].(string)
	const prefix = "*****"
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func selectedValueEqual(a, b interface{}) bool {
	am, _ := a.(map[string]interface{})
	bm, _ := b.(map[string]interface{})
	if len(am) != len(bm) {
		return false
	}
	for k, bv := range bm {
		av, ok := am[k]
		if !ok {
			return false
		}
		// Numeric-tolerant scalar compare; for slices, use looseEqual on items.
		switch bvSlice := bv.(type) {
		case []interface{}:
			avSlice, ok := av.([]interface{})
			if !ok {
				// Fallback: Try []string from MarshalJSON serialization.
				if as, ok := av.([]string); ok {
					avSlice = make([]interface{}, len(as))
					for i, s := range as {
						avSlice[i] = s
					}
				} else {
					return false
				}
			}
			if len(avSlice) != len(bvSlice) {
				return false
			}
			for i := range bvSlice {
				if !looseEqual(avSlice[i], bvSlice[i]) {
					return false
				}
			}
		default:
			if !looseEqual(av, bv) {
				return false
			}
		}
	}
	return true
}

// --- initialization-timeout helper -----------------------------------------

// assertInitializationTimeoutError builds a real Client whose initial fetch
// blocks past initTimeoutSec, then asserts the resolver call for `key` returns
// (or doesn't return, per onInitFailure) the initialization-timeout error.
func assertInitializationTimeoutError(t *testing.T, key string, initTimeoutSec float64, apiURL string, onInitFailure string) {
	t.Helper()

	timeout := time.Duration(initTimeoutSec * float64(time.Second))
	if timeout <= 0 {
		timeout = 1 * time.Millisecond
	}

	// HTTP client whose roundtrips block longer than the init timeout.
	httpClient := &http.Client{
		Transport: blockingRoundTripper(timeout * 50),
	}

	policy := quonfig.ReturnError
	switch onInitFailure {
	case "raise", ":raise":
		policy = quonfig.ReturnError
	case "return", ":return":
		policy = quonfig.ReturnZeroValue
	}

	client, err := quonfig.NewClient(
		quonfig.WithAPIKey("test-key"),
		quonfig.WithAPIURLs([]string{apiURL}),
		quonfig.WithHTTPClient(httpClient),
		quonfig.WithInitTimeout(timeout),
		quonfig.WithOnInitFailure(policy),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	defer client.Close()

	_, _, err = client.GetStringValue(key, nil)
	if policy == quonfig.ReturnError {
		if !errors.Is(err, quonfig.ErrInitializationTimeout) {
			t.Fatalf("expected ErrInitializationTimeout, got %v", err)
		}
	} else {
		// For :return policy, the SDK should NOT raise; the call should return
		// zero value with a non-timeout error (or no error).
		if errors.Is(err, quonfig.ErrInitializationTimeout) {
			t.Fatalf("expected no init-timeout error with :return policy, got %v", err)
		}
	}
}

type roundTripFn func(*http.Request) (*http.Response, error)

func (f roundTripFn) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func blockingRoundTripper(block time.Duration) http.RoundTripper {
	return roundTripFn(func(r *http.Request) (*http.Response, error) {
		select {
		case <-time.After(block):
			return nil, fmt.Errorf("simulated slow init")
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})
}

// --- shared eval helpers reused by feedEvalSummary -------------------------

// (hasTargetingRules and configTypeToTelemetryType already exist in
// test_helpers_test.go; we rely on them rather than redefining.)

// --- mustJSON -------------------------------------------------------------

func mustJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// --- log-level type guard -------------------------------------------------

// isLogLevelConfig reports whether the config is a log-level config. Log-level
// evaluations are intentionally excluded from telemetry.
func isLogLevelConfig(cfg *eval.FullConfig) bool {
	if cfg.Type == quonfig.ConfigTypeLogLevel {
		return true
	}
	return cfg.ValueType == quonfig.ValueTypeLogLevel
}

var _ = eval.EmptyContext{} // ensure the eval package import isn't dropped
