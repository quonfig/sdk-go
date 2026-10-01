package fixtures

// Telemetry support for post_generated_test.go / telemetry_generated_test.go.
//
// A telemetry case builds a REAL datadir-mode client with an SDK key and its
// telemetry URL pointed at a local recorder, drives it through public calls
// (typed getters with the case's contexts), then Close()s it: Close runs the
// SDK's own shutdown flush, so the recorder receives exactly the bytes the
// SDK would send to api-telemetry. Assertions decode those wire bytes
// generically; nothing here computes a reason, a redaction, a shape or an
// example on the SDK's behalf (qfg-2agi.31).

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	quonfig "github.com/quonfig/sdk-go"
)

// telemetryContextProbeKey is the config a context-telemetry case evaluates
// to hand each context record to the SDK. The SDK records the evaluation
// context on every resolved lookup, so any existing key works.
const telemetryContextProbeKey = "brand.new.string"

type telemetryPost struct {
	path string
	body []byte
}

// telemetryHarness owns one real client and the recorder behind its
// telemetry URL.
type telemetryHarness struct {
	Client  *quonfig.Client
	server  *httptest.Server
	mu      sync.Mutex
	posts   []telemetryPost
	drained bool
}

// startTelemetryClient builds the client. overrides are the YAML
// client_overrides (collect_evaluation_summaries, context_upload_mode);
// extra carries client-level options such as WithGlobalContext.
func startTelemetryClient(t *testing.T, overrides map[string]interface{}, extra ...quonfig.Option) *telemetryHarness {
	t.Helper()
	h := &telemetryHarness{}
	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		h.mu.Lock()
		h.posts = append(h.posts, telemetryPost{path: r.URL.Path, body: body})
		h.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(h.server.Close)

	opts := append(baseClientOptions(),
		quonfig.WithSdkKey("itd-telemetry-key"),
		quonfig.WithTelemetryURL(h.server.URL),
		quonfig.WithTelemetrySyncInterval(time.Hour),
	)
	if v, ok := overrides["collect_evaluation_summaries"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			t.Fatalf("collect_evaluation_summaries must be a bool, got %T", v)
		}
		opts = append(opts, quonfig.WithCollectEvaluationSummaries(b))
	}
	if v, ok := overrides["context_upload_mode"]; ok {
		s, _ := v.(string)
		switch strings.TrimPrefix(s, ":") {
		case "none":
			opts = append(opts, quonfig.WithContextTelemetryMode(quonfig.ContextTelemetryNone))
		case "shape_only":
			opts = append(opts, quonfig.WithContextTelemetryMode(quonfig.ContextTelemetryShapes))
		case "periodic_example":
			opts = append(opts, quonfig.WithContextTelemetryMode(quonfig.ContextTelemetryPeriodicExample))
		default:
			t.Fatalf("unknown context_upload_mode %q", s)
		}
	}
	for k := range overrides {
		if k != "collect_evaluation_summaries" && k != "context_upload_mode" {
			t.Fatalf("unsupported telemetry client_override %q", k)
		}
	}
	opts = append(opts, extra...)

	c, err := quonfig.NewClient(opts...)
	if err != nil {
		t.Fatalf("quonfig.NewClient error: %v", err)
	}
	h.Client = c
	t.Cleanup(c.Close)
	return h
}

// events closes the client (the SDK's shutdown flush POSTs the live window)
// and returns every event object the recorder received, in order.
func (h *telemetryHarness) events(t *testing.T) []map[string]interface{} {
	t.Helper()
	if !h.drained {
		h.Client.Close()
		h.drained = true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []map[string]interface{}
	for _, p := range h.posts {
		if strings.TrimSuffix(p.path, "/") != "/api/v1/telemetry" {
			t.Fatalf("telemetry POST went to %q, want /api/v1/telemetry", p.path)
		}
		var payload struct {
			Events []map[string]interface{} `json:"events"`
		}
		if err := json.Unmarshal(p.body, &payload); err != nil {
			t.Fatalf("telemetry POST body is not JSON: %v\n%s", err, p.body)
		}
		out = append(out, payload.Events...)
	}
	return out
}

// eventsOfKind returns the payloads of every event carrying the given wire
// key ("summaries", "contextShapes", "exampleContexts").
func eventsOfKind(events []map[string]interface{}, wireKey string) []map[string]interface{} {
	var out []map[string]interface{}
	for _, e := range events {
		if v, ok := e[wireKey].(map[string]interface{}); ok {
			out = append(out, v)
		}
	}
	return out
}

// assertTelemetryPost asserts the wire payload for one aggregator kind
// matches the YAML expected_data (nil = nothing of that kind reported).
func assertTelemetryPost(t *testing.T, h *telemetryHarness, kind string, expected interface{}, endpoint string) {
	t.Helper()
	// Every kind travels in the unified /api/v1/telemetry POST. The YAML's
	// legacy "/api/v1/context-shapes" endpoint for context_shape names the
	// event, not a separate request.
	switch endpoint {
	case "/api/v1/telemetry", "/api/v1/context-shapes":
	default:
		t.Fatalf("unknown telemetry endpoint %q", endpoint)
	}
	events := h.events(t)
	switch kind {
	case "evaluation_summary":
		assertEvalSummaryWire(t, eventsOfKind(events, "summaries"), expected)
	case "context_shape":
		assertContextShapeWire(t, eventsOfKind(events, "contextShapes"), expected)
	case "example_contexts":
		assertExampleContextsWire(t, eventsOfKind(events, "exampleContexts"), expected)
	default:
		t.Fatalf("unknown aggregator kind %q", kind)
	}
}

// --- evaluation summaries -------------------------------------------------

type summaryRow struct {
	Key                   string
	Type                  string
	Count                 int64
	Reason                int64
	ConfigRowIndex        int64
	ConditionalValueIndex int64
	WeightedValueIndex    int64
	SelectedValue         string // canonical JSON of the {"<type>": value} wrapper
}

func (r summaryRow) String() string {
	return fmt.Sprintf("key=%s type=%s count=%d reason=%d row=%d cond=%d weighted=%d selected=%s",
		r.Key, r.Type, r.Count, r.Reason, r.ConfigRowIndex, r.ConditionalValueIndex, r.WeightedValueIndex, r.SelectedValue)
}

func assertEvalSummaryWire(t *testing.T, summaries []map[string]interface{}, expected interface{}) {
	t.Helper()
	var got []summaryRow
	for _, s := range summaries {
		list, _ := s["summaries"].([]interface{})
		for _, item := range list {
			sm, _ := item.(map[string]interface{})
			key, _ := sm["key"].(string)
			typ, _ := sm["type"].(string)
			counters, _ := sm["counters"].([]interface{})
			for _, ci := range counters {
				c, _ := ci.(map[string]interface{})
				got = append(got, summaryRow{
					Key:                   key,
					Type:                  strings.ToUpper(typ),
					Count:                 asInt(c["count"]),
					Reason:                asInt(c["reason"]),
					ConfigRowIndex:        asInt(c["configRowIndex"]),
					ConditionalValueIndex: asInt(c["conditionalValueIndex"]),
					WeightedValueIndex:    asInt(c["weightedValueIndex"]),
					SelectedValue:         string(mustCanonicalJSON(c["selectedValue"])),
				})
			}
		}
	}

	if expected == nil {
		if len(got) != 0 {
			t.Errorf("expected no evaluation summaries, got:\n%s", rowsString(got))
		}
		return
	}
	list, ok := expected.([]interface{})
	if !ok {
		t.Fatalf("evaluation_summary expected_data must be a list, got %T", expected)
	}
	var want []summaryRow
	for _, item := range list {
		m, _ := item.(map[string]interface{})
		sum, _ := m["summary"].(map[string]interface{})
		sv, hasSV := m["selected_value"]
		if !hasSV {
			t.Fatalf("evaluation_summary expected row has no selected_value: %v", m)
		}
		key, _ := m["key"].(string)
		typ, _ := m["type"].(string)
		want = append(want, summaryRow{
			Key:                   key,
			Type:                  strings.ToUpper(typ),
			Count:                 asInt(m["count"]),
			Reason:                asInt(m["reason"]),
			ConfigRowIndex:        asInt(sum["config_row_index"]),
			ConditionalValueIndex: asInt(sum["conditional_value_index"]),
			WeightedValueIndex:    asInt(sum["weighted_value_index"]),
			SelectedValue:         string(mustCanonicalJSON(normalizeJSON(sv))),
		})
	}
	sortRows(got)
	sortRows(want)
	if rowsString(got) != rowsString(want) {
		t.Errorf("evaluation summary mismatch:\n  got:\n%s\n  want:\n%s", rowsString(got), rowsString(want))
	}
}

func sortRows(rows []summaryRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].String() < rows[j].String() })
}

func rowsString(rows []summaryRow) string {
	parts := make([]string, len(rows))
	for i, r := range rows {
		parts[i] = "    " + r.String()
	}
	return strings.Join(parts, "\n")
}

// --- context shapes -------------------------------------------------------

func assertContextShapeWire(t *testing.T, shapeEvents []map[string]interface{}, expected interface{}) {
	t.Helper()
	got := map[string]map[string]int64{}
	for _, e := range shapeEvents {
		shapes, _ := e["shapes"].([]interface{})
		for _, si := range shapes {
			s, _ := si.(map[string]interface{})
			name, _ := s["name"].(string)
			if _, dup := got[name]; dup {
				t.Errorf("context shape %q reported more than once", name)
			}
			ft := map[string]int64{}
			fields, _ := s["fieldTypes"].(map[string]interface{})
			for k, v := range fields {
				ft[k] = asInt(v)
			}
			got[name] = ft
		}
	}
	if expected == nil {
		if len(got) != 0 {
			t.Errorf("expected no context shapes, got %s", mustJSON(got))
		}
		return
	}
	list, ok := expected.([]interface{})
	if !ok {
		t.Fatalf("context_shape expected_data must be a list, got %T", expected)
	}
	want := map[string]map[string]int64{}
	for _, item := range list {
		m, _ := item.(map[string]interface{})
		name, _ := m["name"].(string)
		ft := map[string]int64{}
		fields, _ := m["field_types"].(map[string]interface{})
		for k, v := range fields {
			ft[k] = asInt(v)
		}
		want[name] = ft
	}
	if mustJSON(got) != mustJSON(want) {
		t.Errorf("context shape mismatch:\n  got:  %s\n  want: %s", mustJSON(got), mustJSON(want))
	}
}

// --- example contexts -----------------------------------------------------

func assertExampleContextsWire(t *testing.T, exampleEvents []map[string]interface{}, expected interface{}) {
	t.Helper()
	var got []map[string]interface{}
	for _, e := range exampleEvents {
		examples, _ := e["examples"].([]interface{})
		for _, ei := range examples {
			ex, _ := ei.(map[string]interface{})
			cs, _ := ex["contextSet"].(map[string]interface{})
			contexts, _ := cs["contexts"].([]interface{})
			flat := map[string]interface{}{}
			for _, ci := range contexts {
				c, _ := ci.(map[string]interface{})
				typ, _ := c["type"].(string)
				flat[typ] = c["values"]
			}
			got = append(got, flat)
		}
	}
	if expected == nil {
		if len(got) != 0 {
			t.Errorf("expected no example contexts, got %s", mustJSON(got))
		}
		return
	}
	want, ok := expected.(map[string]interface{})
	if !ok {
		t.Fatalf("example_contexts expected_data must be a map, got %T", expected)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly one example context, got %d: %s", len(got), mustJSON(got))
	}
	if string(mustCanonicalJSON(normalizeJSON(got[0]))) != string(mustCanonicalJSON(normalizeJSON(want))) {
		t.Errorf("example context mismatch:\n  got:  %s\n  want: %s", mustJSON(got[0]), mustJSON(want))
	}
}

// --- value helpers --------------------------------------------------------

// normalizeJSON round-trips a value through encoding/json so Go literals
// (int) and decoded wire values (float64) compare equal.
func normalizeJSON(v interface{}) interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}

func asInt(v interface{}) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	}
	return 0
}
