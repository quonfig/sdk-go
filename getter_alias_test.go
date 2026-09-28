package quonfig

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

// qfg-9dxb.6 (audit HIGH 3): getters returning a slice or map used to hand
// the caller the same object held in the shared config store. A caller that
// mutated it changed the config for every other caller (and concurrent
// mutation could hit Go's unrecoverable "concurrent map writes"). Each call
// now returns its own copy.

const aliasEnvelope = `{"configs":[` +
	`{"key":"list","type":"config","valueType":"string_list","default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"string_list","value":["b","a"]}}]}},` +
	`{"key":"obj","type":"config","valueType":"json","default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"json","value":{"k":"orig","nested":{"n":1},"arr":[1,{"x":"y"}]}}}]}}` +
	`],"meta":{"version":"gen-1","environment":"Production","generation":1}}`

func newAliasTestClient(t *testing.T) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(aliasEnvelope))
	}))
	t.Cleanup(srv.Close)
	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(false),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	// Let background workers start before Close runs in Cleanup (a separate,
	// known init/Close race is not addressed here).
	waitFor(t, 3*time.Second, client.FallbackPollerActive, "background workers never started")
	return client
}

func TestGetStringSliceValueReturnsCopy(t *testing.T) {
	client := newAliasTestClient(t)
	l, _, err := client.GetStringSliceValue("list", nil)
	if err != nil {
		t.Fatalf("GetStringSliceValue: %v", err)
	}
	l[0] = "MUTATED"
	again, _, _ := client.GetStringSliceValue("list", nil)
	if want := []string{"b", "a"}; !reflect.DeepEqual(again, want) {
		t.Fatalf("second read = %v, want %v (caller mutation leaked into the store)", again, want)
	}
}

func TestGetJSONValueReturnsDeepCopy(t *testing.T) {
	client := newAliasTestClient(t)
	v, _, err := client.GetJSONValue("obj", nil)
	if err != nil {
		t.Fatalf("GetJSONValue: %v", err)
	}
	m := v.(map[string]interface{})
	m["k"] = "MUTATED"
	m["nested"].(map[string]interface{})["n"] = "MUTATED"
	m["arr"].([]interface{})[1].(map[string]interface{})["x"] = "MUTATED"

	again, _, _ := client.GetJSONValue("obj", nil)
	want := map[string]interface{}{
		"k":      "orig",
		"nested": map[string]interface{}{"n": float64(1)},
		"arr":    []interface{}{float64(1), map[string]interface{}{"x": "y"}},
	}
	if !reflect.DeepEqual(again, want) {
		t.Fatalf("second read = %v, want %v (caller mutation leaked into the store)", again, want)
	}
}

func TestEvaluateDetailsAndEvaluateKeyReturnCopies(t *testing.T) {
	client := newAliasTestClient(t)

	d := client.EvaluateDetails("obj", nil)
	d.Value.Value.(map[string]interface{})["k"] = "MUTATED"
	v, _, _, err := client.EvaluateKey("list", nil)
	if err != nil {
		t.Fatalf("EvaluateKey: %v", err)
	}
	v.Value.([]string)[0] = "MUTATED"

	if got, _, _ := client.GetJSONValue("obj", nil); got.(map[string]interface{})["k"] != "orig" {
		t.Errorf("EvaluateDetails mutation leaked: obj = %v", got)
	}
	if got, _, _ := client.GetStringSliceValue("list", nil); got[0] != "b" {
		t.Errorf("EvaluateKey mutation leaked: list = %v", got)
	}
}
