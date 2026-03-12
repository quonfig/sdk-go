package store

import (
	"testing"

	quonfig "github.com/quonfig/sdk-go"
)

func TestStoreEmpty(t *testing.T) {
	s := New()
	if s.Len() != 0 {
		t.Errorf("expected empty store, got %d", s.Len())
	}
	if s.Version() != "" {
		t.Errorf("expected empty version, got %q", s.Version())
	}
	if _, ok := s.Get("anything"); ok {
		t.Error("expected Get to return false for empty store")
	}
	if keys := s.Keys(); len(keys) != 0 {
		t.Errorf("expected no keys, got %v", keys)
	}
}

func TestStoreUpdate(t *testing.T) {
	s := New()

	envelope := &quonfig.ConfigEnvelope{
		Configs: []quonfig.ConfigResponse{
			{
				Key:       "feature.enabled",
				Type:      quonfig.ConfigTypeFeatureFlag,
				ValueType: quonfig.ValueTypeBool,
				Default: quonfig.RuleSet{
					Rules: []quonfig.Rule{
						{Value: quonfig.Value{Type: quonfig.ValueTypeBool, Value: true}},
					},
				},
			},
			{
				Key:       "app.name",
				Type:      quonfig.ConfigTypeConfig,
				ValueType: quonfig.ValueTypeString,
				Default: quonfig.RuleSet{
					Rules: []quonfig.Rule{
						{Value: quonfig.Value{Type: quonfig.ValueTypeString, Value: "myapp"}},
					},
				},
			},
		},
		Meta: quonfig.Meta{
			Version:     "abc123",
			Environment: "production",
		},
	}

	s.Update(envelope)

	if s.Len() != 2 {
		t.Errorf("expected 2 configs, got %d", s.Len())
	}
	if s.Version() != "abc123" {
		t.Errorf("expected version abc123, got %q", s.Version())
	}

	cfg, ok := s.Get("feature.enabled")
	if !ok {
		t.Fatal("expected to find feature.enabled")
	}
	if cfg.ValueType != quonfig.ValueTypeBool {
		t.Errorf("expected bool value type, got %s", cfg.ValueType)
	}
	if !cfg.Default.Rules[0].Value.BoolValue() {
		t.Error("expected feature.enabled to be true")
	}

	cfg, ok = s.Get("app.name")
	if !ok {
		t.Fatal("expected to find app.name")
	}
	if cfg.Default.Rules[0].Value.StringValue() != "myapp" {
		t.Errorf("expected app.name=myapp, got %q", cfg.Default.Rules[0].Value.StringValue())
	}

	if _, ok := s.Get("nonexistent"); ok {
		t.Error("expected Get to return false for nonexistent key")
	}
}

func TestStoreUpdateReplaces(t *testing.T) {
	s := New()

	s.Update(&quonfig.ConfigEnvelope{
		Configs: []quonfig.ConfigResponse{
			{Key: "old.key", ValueType: quonfig.ValueTypeString},
		},
		Meta: quonfig.Meta{Version: "v1"},
	})

	if _, ok := s.Get("old.key"); !ok {
		t.Fatal("expected old.key after first update")
	}

	s.Update(&quonfig.ConfigEnvelope{
		Configs: []quonfig.ConfigResponse{
			{Key: "new.key", ValueType: quonfig.ValueTypeString},
		},
		Meta: quonfig.Meta{Version: "v2"},
	})

	if _, ok := s.Get("old.key"); ok {
		t.Error("expected old.key to be gone after second update")
	}
	if _, ok := s.Get("new.key"); !ok {
		t.Fatal("expected new.key after second update")
	}
	if s.Version() != "v2" {
		t.Errorf("expected version v2, got %q", s.Version())
	}
}

func TestStoreKeys(t *testing.T) {
	s := New()
	s.Update(&quonfig.ConfigEnvelope{
		Configs: []quonfig.ConfigResponse{
			{Key: "a"},
			{Key: "b"},
			{Key: "c"},
		},
	})

	keys := s.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}

	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for _, expected := range []string{"a", "b", "c"} {
		if !keySet[expected] {
			t.Errorf("expected key %q in Keys()", expected)
		}
	}
}
