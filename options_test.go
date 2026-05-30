package quonfig

import (
	"reflect"
	"testing"
	"time"
)

func TestApplyAPIKeyEnvOverride(t *testing.T) {
	t.Run("explicit WithSdkKey wins over env var", func(t *testing.T) {
		t.Setenv("QUONFIG_BACKEND_SDK_KEY", "env-key")
		o := defaultOptions()
		if err := WithSdkKey("explicit-key")(&o); err != nil {
			t.Fatalf("WithSdkKey returned error: %v", err)
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

// TestFallbackPollEnabledByDefault pins the SDK-family-wide default that
// fallback polling is on with a 60s interval. sdk-node/python/ruby/java all
// default to enabled; sdk-go was the outlier prior to qfg-wb2n
// (see project/plans/sdk-1.0-unification.md, Section 1). A NewClient() with
// no options must wire the Layer 2 poller so an SSE-only deployment that
// loses the stream still recovers via polling instead of going silently
// stale.
func TestFallbackPollEnabledByDefault(t *testing.T) {
	t.Run("defaultOptions sets enabled+60s", func(t *testing.T) {
		o := defaultOptions()
		if !o.FallbackPollEnabled {
			t.Errorf("FallbackPollEnabled = false, want true")
		}
		if got, want := o.FallbackPollInterval, 60*time.Second; got != want {
			t.Errorf("FallbackPollInterval = %s, want %s", got, want)
		}
	})

	t.Run("NewClient with no fallback-poll option keeps defaults", func(t *testing.T) {
		client, err := NewClient(
			WithAPIURLs([]string{"https://example.test"}),
			WithAllTelemetryDisabled(),
			WithInitTimeout(50*time.Millisecond),
		)
		if err != nil {
			t.Fatalf("NewClient: %v", err)
		}
		t.Cleanup(client.Close)
		if !client.opts.FallbackPollEnabled {
			t.Errorf("client.opts.FallbackPollEnabled = false, want true")
		}
		if got, want := client.opts.FallbackPollInterval, 60*time.Second; got != want {
			t.Errorf("client.opts.FallbackPollInterval = %s, want %s", got, want)
		}
	})

	t.Run("WithFallbackPoll(false, 0) still disables", func(t *testing.T) {
		o := defaultOptions()
		if err := WithFallbackPoll(false, 0)(&o); err != nil {
			t.Fatalf("WithFallbackPoll(false, 0): %v", err)
		}
		if o.FallbackPollEnabled {
			t.Errorf("FallbackPollEnabled = true, want false after explicit disable")
		}
	})
}

// TestContextTelemetryShapesConstant pins the wire value for the shape-only
// context-telemetry mode. The SDK family agreed on "shapes_only" in qfg-6svs;
// see project/plans/sdk-1.0-unification.md (Section 1).
func TestContextTelemetryShapesConstant(t *testing.T) {
	if got, want := string(ContextTelemetryShapes), "shapes_only"; got != want {
		t.Errorf("ContextTelemetryShapes = %q, want %q", got, want)
	}
	if got, want := string(ContextTelemetryPeriodicExample), "periodic_example"; got != want {
		t.Errorf("ContextTelemetryPeriodicExample = %q, want %q", got, want)
	}
	if got, want := string(ContextTelemetryNone), ""; got != want {
		t.Errorf("ContextTelemetryNone = %q, want %q", got, want)
	}
}
