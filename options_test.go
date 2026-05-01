package quonfig

import (
	"reflect"
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

// TestQuonfigDomainEnvVar verifies that QUONFIG_DOMAIN, when set, derives
// the api, sse (via stream URL), and telemetry URLs uniformly. Mirrors the
// CLI's domain-urls.ts convention so a single env var governs all SDK
// endpoints. Resolution order: explicit With* > QUONFIG_DOMAIN > default.
func TestQuonfigDomainEnvVar(t *testing.T) {
	t.Run("default with no env var resolves to prod", func(t *testing.T) {
		// Explicitly clear in case ambient env has it set.
		t.Setenv("QUONFIG_DOMAIN", "")
		o := defaultOptions()
		applyDomainEnvOverride(&o)
		wantAPI := []string{
			"https://primary.quonfig.com",
			"https://secondary.quonfig.com",
		}
		if !reflect.DeepEqual(o.APIURLs, wantAPI) {
			t.Errorf("APIURLs = %v, want %v", o.APIURLs, wantAPI)
		}
		if got, want := o.TelemetryURL, "https://telemetry.quonfig.com"; got != want {
			t.Errorf("TelemetryURL = %q, want %q", got, want)
		}
	})

	t.Run("QUONFIG_DOMAIN derives all three URLs", func(t *testing.T) {
		t.Setenv("QUONFIG_DOMAIN", "quonfig-staging.com")
		o := defaultOptions()
		applyDomainEnvOverride(&o)
		wantAPI := []string{
			"https://primary.quonfig-staging.com",
			"https://secondary.quonfig-staging.com",
		}
		if !reflect.DeepEqual(o.APIURLs, wantAPI) {
			t.Errorf("APIURLs = %v, want %v", o.APIURLs, wantAPI)
		}
		if got, want := o.TelemetryURL, "https://telemetry.quonfig-staging.com"; got != want {
			t.Errorf("TelemetryURL = %q, want %q", got, want)
		}
	})

	t.Run("explicit WithTelemetryURL wins over QUONFIG_DOMAIN", func(t *testing.T) {
		t.Setenv("QUONFIG_DOMAIN", "quonfig-staging.com")
		o := defaultOptions()
		if err := WithTelemetryURL("http://localhost:6555")(&o); err != nil {
			t.Fatalf("WithTelemetryURL returned error: %v", err)
		}
		applyDomainEnvOverride(&o)
		if got, want := o.TelemetryURL, "http://localhost:6555"; got != want {
			t.Errorf("TelemetryURL = %q, want %q", got, want)
		}
	})

	t.Run("explicit WithAPIURLs wins over QUONFIG_DOMAIN", func(t *testing.T) {
		t.Setenv("QUONFIG_DOMAIN", "quonfig-staging.com")
		o := defaultOptions()
		if err := WithAPIURLs([]string{"http://localhost:8080"})(&o); err != nil {
			t.Fatalf("WithAPIURLs returned error: %v", err)
		}
		applyDomainEnvOverride(&o)
		wantAPI := []string{"http://localhost:8080"}
		if !reflect.DeepEqual(o.APIURLs, wantAPI) {
			t.Errorf("APIURLs = %v, want %v", o.APIURLs, wantAPI)
		}
	})

	t.Run("NewClient end-to-end with QUONFIG_DOMAIN", func(t *testing.T) {
		t.Setenv("QUONFIG_DOMAIN", "quonfig-staging.com")
		t.Setenv("QUONFIG_BACKEND_SDK_KEY", "")
		client, err := NewClient()
		if err != nil {
			t.Fatalf("NewClient returned error: %v", err)
		}
		wantAPI := []string{
			"https://primary.quonfig-staging.com",
			"https://secondary.quonfig-staging.com",
		}
		if !reflect.DeepEqual(client.opts.APIURLs, wantAPI) {
			t.Errorf("APIURLs = %v, want %v", client.opts.APIURLs, wantAPI)
		}
		if got, want := client.opts.TelemetryURL, "https://telemetry.quonfig-staging.com"; got != want {
			t.Errorf("TelemetryURL = %q, want %q", got, want)
		}
	})
}
