package quonfig

import (
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/quonfig/sdk-go/internal/telemetry"
)

// Telemetry must not keep references to caller-owned nested context values
// (qfg-goi1.2.4 item 1). The submitter aggregates contexts on its own
// goroutine and json.Marshals them at flush time; if a nested map is still the
// caller's object, a caller write after the call returns races that marshal and
// Go aborts the whole process with "concurrent map iteration and map write".

type telemetryTestStruct struct{ Tags map[string]string }

func TestContextSetToTelemetryData_DeepCopiesNestedValues(t *testing.T) {
	inner := map[string]interface{}{"deep": "a"}
	nested := map[string]interface{}{"plan": "pro", "inner": inner}
	list := []interface{}{"x", map[string]interface{}{"y": 1}}
	strs := []string{"s1", "s2"}

	cs := NewContextSet().WithNamedContextValues("user", map[string]interface{}{
		"key":    "u-1",
		"age":    42,
		"meta":   nested,
		"list":   list,
		"strs":   strs,
		"custom": map[string]string{"k": "v"},
		"ptr":    &telemetryTestStruct{Tags: map[string]string{"t": "1"}},
		"struct": telemetryTestStruct{Tags: map[string]string{"t": "1"}},
	})

	data := contextSetToTelemetryData(cs)

	// Caller mutates everything it passed in after the call returned.
	nested["plan"] = "mutated"
	inner["deep"] = "mutated"
	list[0] = "mutated"
	list[1].(map[string]interface{})["y"] = 2
	strs[0] = "mutated"

	props := data.Contexts["user"]
	if props["key"] != "u-1" || props["age"] != 42 {
		t.Fatalf("scalars not passed through: %#v", props)
	}
	meta, _ := props["meta"].(map[string]interface{})
	if meta["plan"] != "pro" {
		t.Errorf("meta.plan = %v, want pro (telemetry shares the caller's map)", meta["plan"])
	}
	if got := meta["inner"].(map[string]interface{})["deep"]; got != "a" {
		t.Errorf("meta.inner.deep = %v, want a (telemetry shares the caller's nested map)", got)
	}
	gotList, _ := props["list"].([]interface{})
	if gotList[0] != "x" || gotList[1].(map[string]interface{})["y"] != 1 {
		t.Errorf("list = %#v, want copy of [x {y:1}]", gotList)
	}
	if gotStrs, _ := props["strs"].([]string); gotStrs[0] != "s1" {
		t.Errorf("strs = %#v, want copy of [s1 s2]", gotStrs)
	}
	for _, k := range []string{"custom", "ptr", "struct"} {
		if v, ok := props[k]; ok {
			t.Errorf("%s = %#v: non-JSON reference kinds must be dropped from telemetry", k, v)
		}
	}
}

// telemetryTestUUID mirrors google/uuid's uuid.UUID: a [16]byte array with a
// value-receiver MarshalText, which json.Marshal renders as a string.
type telemetryTestUUID [16]byte

func (u telemetryTestUUID) MarshalText() ([]byte, error) {
	return []byte(hex.EncodeToString(u[:])), nil
}

// Value kinds are not shared with the caller, so they must survive the
// telemetry copy (qfg-goi1.2.44). A uuid.UUID-style key was dropped, which made
// the example context look key-less and discarded it entirely.
func TestContextSetToTelemetryData_KeepsArraysAndTextMarshalers(t *testing.T) {
	id := telemetryTestUUID{0xde, 0xad, 0xbe, 0xef}
	cs := NewContextSet().WithNamedContextValues("user", map[string]interface{}{
		"key":     id,
		"ids":     [2]int{1, 2},
		"idPtr":   &id,
		"structs": [1]telemetryTestStruct{},
	})

	data := contextSetToTelemetryData(cs)

	props := data.Contexts["user"]
	if got, want := props["key"], hex.EncodeToString(id[:]); got != want {
		t.Errorf("key = %#v, want %q (TextMarshaler pre-marshalled to a string)", got, want)
	}
	if got, _ := json.Marshal(props["ids"]); string(got) != "[1,2]" {
		t.Errorf("ids = %#v (json %s), want [1,2]", props["ids"], got)
	}
	for _, k := range []string{"idPtr", "structs"} {
		if v, ok := props[k]; ok {
			t.Errorf("%s = %#v: pointers and arrays of non-scalars must stay dropped", k, v)
		}
	}

	agg := telemetry.NewExampleContextAggregator()
	agg.Record(data)
	ev := agg.GetAndClear()
	if ev == nil || ev.ExampleContexts == nil || len(ev.ExampleContexts.Examples) != 1 {
		t.Fatalf("example context with a uuid key was discarded: %#v", ev)
	}
	values := ev.ExampleContexts.Examples[0].ContextSet.Contexts[0].Values
	if got, _ := json.Marshal(values); string(got) != `{"ids":[1,2],"key":"`+hex.EncodeToString(id[:])+`"}` {
		t.Errorf("example context values = %s", got)
	}
}

// The bead's own repro (qfg-goi1.2.44): {key: uuid.UUID, ids: []int{1,2}}.
// v1.5.0 sent {"ids":[1,2],"key":"<uuid>"}. A slice of scalars is a reference
// kind, so it must be copied, not dropped: the copy shares nothing with the
// caller and keeps the slice's own type, so the flushed JSON matches v1.5.0.
type telemetryTestIDs []int64

func TestContextSetToTelemetryData_CopiesScalarSlices(t *testing.T) {
	id := telemetryTestUUID{0xca, 0xfe}
	ids := []int{1, 2}
	i64s := telemetryTestIDs{3, 4}
	raw := []byte("hi")
	cs := NewContextSet().WithNamedContextValues("user", map[string]interface{}{
		"key":   id,
		"ids":   ids,
		"i64s":  i64s,
		"bytes": raw,
		"nil":   []int(nil),
		"tags":  map[string]string{"t": "1"},
		"maps":  []map[string]interface{}{{"a": 1}},
		"nest":  [][]int{{1}},
	})

	data := contextSetToTelemetryData(cs)

	// Caller writes to everything it passed in after the call returned.
	ids[0] = 99
	i64s[0] = 99
	raw[0] = 'X'

	props := data.Contexts["user"]
	if got, _ := json.Marshal(props["ids"]); string(got) != "[1,2]" {
		t.Errorf("ids = %#v (json %s), want a copy of [1,2]", props["ids"], got)
	}
	if got, ok := props["i64s"].(telemetryTestIDs); !ok || got[0] != 3 || got[1] != 4 {
		t.Errorf("i64s = %#v, want a telemetryTestIDs copy of [3 4]", props["i64s"])
	}
	if got, _ := props["bytes"].([]byte); string(got) != "hi" {
		t.Errorf("bytes = %#v, want a copy of \"hi\"", props["bytes"])
	}
	if got, ok := props["nil"]; !ok || got.([]int) != nil {
		t.Errorf("nil = %#v, want a nil []int", got)
	}
	for _, k := range []string{"tags", "maps", "nest"} {
		if v, ok := props[k]; ok {
			t.Errorf("%s = %#v: maps and slices of non-scalars must stay dropped", k, v)
		}
	}

	// The full converted context, dropped keys included, reaches the example
	// aggregator as one reportable example.
	agg := telemetry.NewExampleContextAggregator()
	agg.Record(data)
	ev := agg.GetAndClear()
	if ev == nil || ev.ExampleContexts == nil || len(ev.ExampleContexts.Examples) != 1 {
		t.Fatalf("example context was discarded: %#v", ev)
	}
	values := ev.ExampleContexts.Examples[0].ContextSet.Contexts[0].Values
	want := `{"bytes":"aGk=","i64s":[3,4],"ids":[1,2],"key":"` + hex.EncodeToString(id[:]) + `","nil":null}`
	if got, _ := json.Marshal(values); string(got) != want {
		t.Errorf("example context values = %s, want %s", got, want)
	}
}

// telemetryTestBothMarshaler implements json.Marshaler and
// encoding.TextMarshaler with different output. json.Marshal (what v1.5.0 ran
// at flush time) prefers MarshalJSON, so telemetry must record that output.
type telemetryTestBothMarshaler struct{ s string }

func (b telemetryTestBothMarshaler) MarshalJSON() ([]byte, error) {
	if b.s != "" {
		return json.Marshal("json-" + b.s)
	}
	return []byte(`{"j":1}`), nil
}

func (b telemetryTestBothMarshaler) MarshalText() ([]byte, error) {
	return []byte("text"), nil
}

func TestContextSetToTelemetryData_PrefersMarshalJSON(t *testing.T) {
	cs := NewContextSet().WithNamedContextValues("user", map[string]interface{}{
		"key": telemetryTestBothMarshaler{s: "k"},
		"obj": telemetryTestBothMarshaler{},
	})

	props := contextSetToTelemetryData(cs).Contexts["user"]
	if got := props["key"]; got != "json-k" {
		t.Errorf("key = %#v, want the MarshalJSON string \"json-k\"", got)
	}
	if got, _ := json.Marshal(props["obj"]); string(got) != `{"j":1}` {
		t.Errorf("obj = %#v (json %s), want the MarshalJSON output {\"j\":1}", props["obj"], got)
	}
}

// End-to-end form of the audit repro: a telemetry-on client evaluates with a
// nested context value, then the caller keeps writing to it. Before the fix
// this failed under -race (json.mapEncoder vs the caller's write) and, without
// -race, killed the test binary with a fatal error.
func TestTelemetryNestedContextMutationDoesNotRace(t *testing.T) {
	t.Setenv("QUONFIG_BACKEND_SDK_KEY", "")
	rec := newTelemetryPostRecorder(t)

	client, err := NewClient(
		WithSdkKey("test-backend-key"),
		WithDataDir(telemetryWorkspaceFixture(t)),
		WithEnvironment("Production"),
		WithTelemetryURL(rec.server.URL),
		WithTelemetrySyncInterval(20*time.Millisecond),
		WithContextTelemetryMode(ContextTelemetryPeriodicExample),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	deadline := time.Now().Add(400 * time.Millisecond)
	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			// One nested map per worker, reused across calls and written to
			// after every call: ordinary Go once the call has returned.
			nested := map[string]interface{}{"n": 0}
			for i := 0; time.Now().Before(deadline); i++ {
				ctx := NewContextSet().WithNamedContextValues("user", map[string]interface{}{
					"key":  w*1_000_000 + i, // distinct key so the example aggregator keeps it
					"meta": nested,
				})
				if _, _, err := client.GetStringValue("welcome-message", ctx); err != nil {
					t.Errorf("GetStringValue: %v", err)
					return
				}
				for j := 0; j < 200; j++ {
					nested["n"] = i + j
				}
				time.Sleep(100 * time.Microsecond)
			}
		}(w)
	}
	wg.Wait()
}

// telemetryTestNullable mirrors google/uuid's uuid.NullUUID: MarshalJSON
// returns null when the value is not set, and MarshalText returns "".
type telemetryTestNullable struct {
	ID    telemetryTestUUID
	Valid bool
}

func (n telemetryTestNullable) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.ID)
}

func (n telemetryTestNullable) MarshalText() ([]byte, error) {
	if !n.Valid {
		return []byte{}, nil
	}
	return n.ID.MarshalText()
}

// json.Marshal (the v1.5.0 flush) sends null for a MarshalJSON that returns
// null, so telemetry must record nil, not the empty string.
func TestContextSetToTelemetryData_MarshalJSONNullStaysNull(t *testing.T) {
	cs := NewContextSet().WithNamedContextValues("user", map[string]interface{}{
		"key":   "u-1",
		"unset": telemetryTestNullable{},
		"set":   telemetryTestNullable{ID: telemetryTestUUID{0xab}, Valid: true},
	})

	props := contextSetToTelemetryData(cs).Contexts["user"]
	if got, ok := props["unset"]; !ok || got != nil {
		t.Errorf("unset = %#v (present %v), want nil (JSON null)", got, ok)
	}
	want := telemetryTestUUID{0xab}
	if got := props["set"]; got != hex.EncodeToString(want[:]) {
		t.Errorf("set = %#v, want the MarshalJSON string", got)
	}
}

// telemetryTestPanicMarshaler panics if anything renders it. With example
// contexts off, v1.5.0 never rendered a context value, so recording a context
// must not run the value's MarshalText or MarshalJSON.
type telemetryTestPanicMarshaler [4]byte

func (telemetryTestPanicMarshaler) MarshalText() ([]byte, error) {
	panic("telemetry rendered a context value with example contexts off")
}

func TestTelemetrySubmitter_RecordContextRendersNothingWithoutExampleContexts(t *testing.T) {
	for _, mode := range []ContextTelemetryMode{ContextTelemetryNone, ContextTelemetryShapes} {
		t.Run("mode="+string(mode), func(t *testing.T) {
			ts := newTelemetrySubmitter(Options{
				APIKey:                     "test-backend-key",
				CollectEvaluationSummaries: true,
				ContextTelemetryMode:       mode,
			})
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("RecordContext panicked on the evaluation path: %v", r)
				}
			}()
			ts.RecordContext(NewContextSet().WithNamedContextValues("user", map[string]interface{}{
				"key": "u-1",
				"id":  telemetryTestPanicMarshaler{1},
			}))
		})
	}
}

// In shapes_only mode, the shapes recorded from the converted context must be
// exactly the ones v1.5.0 recorded from the caller's own values, for every
// kind (v1.5.0 dropped nothing from shapes), and the converted context must
// not hold the caller's nested objects.
func TestContextSetToTelemetryShapes_MatchesV150Shapes(t *testing.T) {
	nested := map[string]interface{}{"a": 1}
	values := map[string]interface{}{
		"key":   "u-1",
		"n":     1,
		"i32":   int32(2),
		"i64":   int64(3),
		"u8":    uint8(4),
		"f32":   float32(1.5),
		"f64":   2.5,
		"b":     true,
		"nil":   nil,
		"time":  time.Unix(0, 0),
		"strs":  []string{"a"},
		"list":  []interface{}{1},
		"meta":  nested,
		"tags":  map[string]string{"t": "1"},
		"ids":   []int{1, 2},
		"uuid":  telemetryTestUUID{1},
		"ptr":   &telemetryTestStruct{},
		"st":    telemetryTestStruct{},
		"panic": telemetryTestPanicMarshaler{},
		"ch":    make(chan int),
	}
	cs := NewContextSet().WithNamedContextValues("user", values)

	data := contextSetToTelemetryShapes(cs)

	want := telemetry.NewContextShapeAggregator()
	want.Record(telemetry.ContextData{Contexts: map[string]map[string]interface{}{"user": values}})
	got := telemetry.NewContextShapeAggregator()
	got.Record(data)
	wantShapes, _ := json.Marshal(want.GetAndClear())
	gotShapes, _ := json.Marshal(got.GetAndClear())
	if string(gotShapes) != string(wantShapes) {
		t.Errorf("shapes = %s, want the v1.5.0 shapes %s", gotShapes, wantShapes)
	}
	if m, ok := data.Contexts["user"]["meta"].(map[string]interface{}); ok && m != nil {
		t.Errorf("meta = %#v: the shapes context must not hold the caller's map", m)
	}
}

// A context value that contains itself must not kill the process
// (qfg-goi1.2.46). The deep copy above recursed with no cycle guard, so a
// self-referential map or slice recursed until Go's stack limit: "fatal error:
// stack overflow", which resolveDetail's recover cannot catch. v1.5.0
// shallow-copied the context and json.Marshal reported the cycle as an ordinary
// error at flush. The copy drops the edge that closes the cycle and keeps the
// rest of the value.
func TestTelemetryCyclicContextValueSurvivesEvaluation(t *testing.T) {
	t.Setenv("QUONFIG_BACKEND_SDK_KEY", "")
	rec := newTelemetryPostRecorder(t)

	client, err := NewClient(
		WithSdkKey("test-backend-key"),
		WithDataDir(telemetryWorkspaceFixture(t)),
		WithEnvironment("Production"),
		WithTelemetryURL(rec.server.URL),
		WithContextTelemetryMode(ContextTelemetryPeriodicExample),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	m := map[string]interface{}{"plan": "pro"}
	m["self"] = m
	ctx := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"key": "u1", "blob": m})
	if _, _, err := client.GetStringValue("welcome-message", ctx); err != nil {
		t.Fatalf("GetStringValue: %v", err)
	}
}

func TestContextSetToTelemetryData_DropsCyclicEdges(t *testing.T) {
	m := map[string]interface{}{"plan": "pro"}
	m["self"] = m
	m["list"] = []interface{}{"a", m}

	s := []interface{}{"x", nil}
	s[1] = s

	// A map that refers to itself many times: dropping only on the current
	// path keeps this linear instead of exponential.
	wide := map[string]interface{}{}
	for i := 0; i < 64; i++ {
		wide[string(rune('a'+i%26))+string(rune('a'+i/26))] = wide
	}

	// The same acyclic map reached twice is not a cycle and is kept both times.
	shared := map[string]interface{}{"k": "v"}

	cs := NewContextSet().WithNamedContextValues("user", map[string]interface{}{
		"key":    "u1",
		"blob":   m,
		"slice":  s,
		"wide":   wide,
		"shared": []interface{}{shared, shared},
	})

	data := contextSetToTelemetryData(cs)
	if _, err := json.Marshal(data.Contexts); err != nil {
		t.Fatalf("json.Marshal(copy): %v", err)
	}

	props := data.Contexts["user"]
	blob, _ := props["blob"].(map[string]interface{})
	if blob["plan"] != "pro" {
		t.Errorf("blob.plan = %v, want pro", blob["plan"])
	}
	if _, ok := blob["self"]; ok {
		t.Errorf("blob.self kept, want the cyclic edge dropped: %#v", blob["self"])
	}
	if l, _ := blob["list"].([]interface{}); len(l) != 1 || l[0] != "a" {
		t.Errorf("blob.list = %#v, want [a]", blob["list"])
	}
	if sl, _ := props["slice"].([]interface{}); len(sl) != 1 || sl[0] != "x" {
		t.Errorf("slice = %#v, want [x]", props["slice"])
	}
	if w, _ := props["wide"].(map[string]interface{}); len(w) != 0 {
		t.Errorf("wide = %#v, want empty map", props["wide"])
	}
	sh, _ := props["shared"].([]interface{})
	if len(sh) != 2 || sh[0].(map[string]interface{})["k"] != "v" || sh[1].(map[string]interface{})["k"] != "v" {
		t.Errorf("shared = %#v, want both copies of {k:v}", props["shared"])
	}
}
