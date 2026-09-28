package quonfig

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// missingHashConfig mirrors integration-test-data feature-flag.weighted: it
// hashes on user.tracking_id and its first variant is 1.
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

// TestEvaluateDetails_MissingHashPropertyServesFirstVariant guards
// qfg-9dxb.8: a weighted rollout whose hash property is absent serves the
// first variant on every call, keeps reason SPLIT, and flags
// hashPropertyMissing in FlagMetadata.
func TestEvaluateDetails_MissingHashPropertyServesFirstVariant(t *testing.T) {
	client := missingHashClient(t, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), "feature-flag.weighted")

	cases := map[string]*ContextSet{
		"no context":            nil,
		"named context missing": NewContextSet().WithNamedContextValues("device", map[string]interface{}{"tracking_id": "d-1"}),
		"property missing":      NewContextSet().WithNamedContextValues("user", map[string]interface{}{"email": "a@b.test"}),
	}
	for name, ctx := range cases {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 50; i++ {
				d := client.EvaluateDetails("feature-flag.weighted", ctx)
				if d.Value == nil || d.Value.Value != int64(1) {
					t.Fatalf("iteration %d: value=%v, want first variant 1", i, d.Value)
				}
				if d.Reason != ReasonSplit || d.Variant != "split:0" {
					t.Fatalf("iteration %d: reason=%v variant=%q, want SPLIT split:0", i, d.Reason, d.Variant)
				}
				if got, ok := d.FlagMetadata["hashPropertyMissing"]; !ok || got != true {
					t.Fatalf("iteration %d: FlagMetadata[hashPropertyMissing]=%v (present=%v), want true", i, got, ok)
				}
			}
		})
	}

	// Property present: normal bucketing, and no hashPropertyMissing key.
	d := client.EvaluateDetails("feature-flag.weighted", NewContextSet().WithNamedContextValues("user", map[string]interface{}{"tracking_id": "user-0"}))
	if d.Value == nil || d.Value.Value != int64(2) {
		t.Fatalf("present: value=%v, want 2 (v1.3.0 bucket for user-0)", d.Value)
	}
	if _, ok := d.FlagMetadata["hashPropertyMissing"]; ok {
		t.Errorf("present: FlagMetadata must not contain hashPropertyMissing, got %v", d.FlagMetadata)
	}
}

// TestMissingHashProperty_WarnsOncePerKey: many concurrent evaluations of two
// affected flags produce exactly one WARN per flag; a present property logs
// nothing.
func TestMissingHashProperty_WarnsOncePerKey(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	client := missingHashClient(t, logger, "flag.a", "flag.b")

	present := NewContextSet().WithNamedContextValues("user", map[string]interface{}{"tracking_id": "user-0"})
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_, _, _ = client.GetIntValue("flag.a", nil)
				_ = client.EvaluateDetails("flag.b", nil)
				_ = client.EvaluateDetails("flag.a", present)
			}
		}()
	}
	wg.Wait()

	out := buf.String()
	for _, key := range []string{"flag.a", "flag.b"} {
		msg := `quonfig: weighted rollout for \"` + key + `\" hashes on \"user.tracking_id\" which is missing from context; using first variant`
		if n := strings.Count(out, msg); n != 1 {
			t.Errorf("WARN for %s logged %d times, want 1; log:\n%s", key, n, out)
		}
	}
	if n := strings.Count(out, "level=WARN"); n != 2 {
		t.Errorf("got %d WARN lines, want 2; log:\n%s", n, out)
	}
}
