package quonfig

import (
	"testing"
)

func TestApplyAPIKeyEnvOverride(t *testing.T) {
	t.Run("explicit WithAPIKey wins over env var", func(t *testing.T) {
		t.Setenv("QUONFIG_BACKEND_SDK_KEY", "env-key")
		o := defaultOptions()
		if err := WithAPIKey("explicit-key")(&o); err != nil {
			t.Fatalf("WithAPIKey returned error: %v", err)
		}
		applyAPIKeyEnvOverride(&o)
		if o.APIKey != "explicit-key" {
			t.Errorf("expected explicit-key, got %q", o.APIKey)
		}
	})

	t.Run("falls back to QUONFIG_BACKEND_SDK_KEY when no option set", func(t *testing.T) {
		t.Setenv("QUONFIG_BACKEND_SDK_KEY", "env-key")
		o := defaultOptions()
		applyAPIKeyEnvOverride(&o)
		if o.APIKey != "env-key" {
			t.Errorf("expected env-key, got %q", o.APIKey)
		}
	})

	t.Run("no-op when env var empty and option unset", func(t *testing.T) {
		t.Setenv("QUONFIG_BACKEND_SDK_KEY", "")
		o := defaultOptions()
		applyAPIKeyEnvOverride(&o)
		if o.APIKey != "" {
			t.Errorf("expected empty APIKey, got %q", o.APIKey)
		}
	})
}

func TestNewClientReadsAPIKeyFromEnv(t *testing.T) {
	t.Setenv("QUONFIG_BACKEND_SDK_KEY", "env-key-xyz")
	client, err := NewClient(WithAPIURLs([]string{"https://example.test"}))
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}
	if client.opts.APIKey != "env-key-xyz" {
		t.Errorf("expected APIKey from env, got %q", client.opts.APIKey)
	}
}
