package quonfig

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// missingHashConfig mirrors integration-test-data feature-flag.weighted: it
// hashes on user.tracking_id with variants 1 (1%), 3 (2%) and 2 (97%).
func missingHashConfig(key string) ConfigResponse {
	return ConfigResponse{
		ID:        "cfg-" + key,
		Key:       key,
		Type:      ConfigTypeFeatureFlag,
		ValueType: ValueTypeInt,
		Default: RuleSet{Rules: []Rule{{
			Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
			Value: Value{
				Type: ValueTypeWeightedValues,
				Value: &WeightedValuesData{
					HashByPropertyName: "user.tracking_id",
					WeightedValues: []WeightedValue{
						{Weight: 1000, Value: Value{Type: ValueTypeInt, Value: int64(1)}},
						{Weight: 2000, Value: Value{Type: ValueTypeInt, Value: int64(3)}},
						{Weight: 97000, Value: Value{Type: ValueTypeInt, Value: int64(2)}},
					},
				},
			},
		}}},
	}
}

func missingHashClient(t *testing.T, logger *slog.Logger, keys ...string) *Client {
	t.Helper()
	client, err := NewClient(WithLogger(logger))
	if err != nil {
		t.Fatal(err)
	}
	configs := make([]ConfigResponse, 0, len(keys))
	for _, k := range keys {
		configs = append(configs, missingHashConfig(k))
	}
	client.installEnvelope(&ConfigEnvelope{
		Meta:    Meta{Version: "v1", Environment: "Production"},
		Configs: configs,
	}, -1)
	return client
}

// TestEvaluateDetails_MissingHashPropertyHashesEmpty guards qfg-9dxb.8: a
// weighted rollout whose hash property is missing hashes an empty value, so
// it serves the same variant as a present "" (value 2 for this fixture, the
// v1.3.0 result), keeps reason SPLIT, and flags hashPropertyMissing in
// FlagMetadata only when the property is missing.
func TestEvaluateDetails_MissingHashPropertyHashesEmpty(t *testing.T) {
	client := missingHashClient(t, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), "feature-flag.weighted")

	empty := client.EvaluateDetails("feature-flag.weighted", NewContextSet().WithNamedContextValues("user", map[string]interface{}{"tracking_id": ""}))
	if empty.Value == nil || empty.Value.Value != int64(2) || empty.Reason != ReasonSplit || empty.Variant != "split:2" {
		t.Fatalf(`present "": value=%v reason=%v variant=%q, want 2 SPLIT split:2`, empty.Value, empty.Reason, empty.Variant)
	}
	if _, ok := empty.FlagMetadata["hashPropertyMissing"]; ok {
		t.Errorf(`present "": FlagMetadata must not contain hashPropertyMissing, got %v`, empty.FlagMetadata)
	}

	cases := map[string]*ContextSet{
		"no context":            nil,
		"empty context":         NewContextSet(),
		"named context missing": NewContextSet().WithNamedContextValues("device", map[string]interface{}{"tracking_id": "d-1"}),
		"property missing":      NewContextSet().WithNamedContextValues("user", map[string]interface{}{"email": "a@b.test"}),
		"value nil":             NewContextSet().WithNamedContextValues("user", map[string]interface{}{"tracking_id": nil}),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 50; i++ {
				d := client.EvaluateDetails("feature-flag.weighted", ctx)
				if d.Value == nil || d.Value.Value != int64(2) {
					t.Fatalf("iteration %d: value=%v, want 2 (same as present \"\")", i, d.Value)
				}
				if d.Reason != ReasonSplit || d.Variant != "split:2" {
					t.Fatalf("iteration %d: reason=%v variant=%q, want SPLIT split:2", i, d.Reason, d.Variant)
				}
				if got, ok := d.FlagMetadata["hashPropertyMissing"]; !ok || got != true {
					t.Fatalf("iteration %d: FlagMetadata[hashPropertyMissing]=%v (present=%v), want true", i, got, ok)
				}
			}
		})
	}

	// Property present: normal bucketing, and no hashPropertyMissing key.
	d := client.EvaluateDetails("feature-flag.weighted", NewContextSet().WithNamedContextValues("user", map[string]interface{}{"tracking_id": "user-71"}))
	if d.Value == nil || d.Value.Value != int64(1) {
		t.Fatalf("present: value=%v, want 1 (v1.3.0 bucket for user-71)", d.Value)
	}
	if _, ok := d.FlagMetadata["hashPropertyMissing"]; ok {
		t.Errorf("present: FlagMetadata must not contain hashPropertyMissing, got %v", d.FlagMetadata)
	}
}

// TestEvaluateDetails_NoHashPropertyIsRandom: a rollout with no hash property
// configured picks a random variant on every evaluation, without the metadata
// or a WARN.
func TestEvaluateDetails_NoHashPropertyIsRandom(t *testing.T) {
	var buf bytes.Buffer
	client := missingHashClient(t, slog.New(slog.NewTextHandler(&buf, nil)), "no-hash")
	client.installEnvelope(&ConfigEnvelope{
		Meta: Meta{Version: "v2", Environment: "Production"},
		Configs: []ConfigResponse{{
			ID: "cfg-no-hash", Key: "no-hash", Type: ConfigTypeFeatureFlag, ValueType: ValueTypeString,
			Default: RuleSet{Rules: []Rule{{
				Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}},
				Value: Value{Type: ValueTypeWeightedValues, Value: &WeightedValuesData{WeightedValues: []WeightedValue{
					{Weight: 50, Value: Value{Type: ValueTypeString, Value: "A"}},
					{Weight: 50, Value: Value{Type: ValueTypeString, Value: "B"}},
				}}},
			}}},
		}},
	}, -1)
	for name, ctx := range map[string]*ContextSet{"no context": nil, "with context": NewContextSet().WithNamedContextValues("user", map[string]interface{}{"key": "u-1"})} {
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			counts := map[string]int{}
			var wg sync.WaitGroup
			for g := 0; g < 10; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < 100; i++ {
						d := client.EvaluateDetails("no-hash", ctx)
						if _, ok := d.FlagMetadata["hashPropertyMissing"]; ok {
							t.Errorf("no hash property: unexpected hashPropertyMissing")
						}
						mu.Lock()
						counts[d.Value.StringValue()]++
						mu.Unlock()
					}
				}()
			}
			wg.Wait()
			t.Logf("1000 evaluations: %v", counts)
			if counts["A"] == 0 || counts["B"] == 0 || counts["A"]+counts["B"] != 1000 {
				t.Fatalf("want both variants over 1000 evaluations, got %v", counts)
			}
		})
	}
	if strings.Contains(buf.String(), "level=WARN") {
		t.Errorf("no hash property must not WARN; log:\n%s", buf.String())
	}
}

// TestMissingHashProperty_WarnsOncePerKey: many concurrent evaluations of two
// affected flags produce exactly one WARN per flag; a present property
// (including an empty one) logs nothing.
func TestMissingHashProperty_WarnsOncePerKey(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := missingHashClient(t, logger, "flag.a", "flag.b")

	present := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"tracking_id": "user-0"})
	empty := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"tracking_id": ""})
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_, _, _ = client.GetIntValue("flag.a", nil)
				_ = client.EvaluateDetails("flag.b", nil)
				_ = client.EvaluateDetails("flag.a", present)
				_ = client.EvaluateDetails("flag.b", empty)
			}
		}()
	}
	wg.Wait()

	out := buf.String()
	for _, key := range []string{"flag.a", "flag.b"} {
		msg := `quonfig: weighted rollout for \"` + key + `\" hashes on \"user.tracking_id\" which is missing from context; hashing an empty value instead`
		if n := strings.Count(out, msg); n != 1 {
			t.Errorf("WARN for %s logged %d times, want 1; log:\n%s", key, n, out)
		}
	}
	if n := strings.Count(out, "level=WARN"); n != 2 {
		t.Errorf("got %d WARN lines, want 2; log:\n%s", n, out)
	}
}
