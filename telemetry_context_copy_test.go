package quonfig

import (
	"sync"
	"testing"
	"time"
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
