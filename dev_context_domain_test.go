package quonfig

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTokensFileNamed(t *testing.T, home, filename, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, ".quonfig"), 0o755); err != nil {
		t.Fatalf("mkdir .quonfig: %v", err)
	}
	if err := os.WriteFile(filepath.Join(home, ".quonfig", filename), []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}

func TestTokenFilenameForAPIURLs(t *testing.T) {
	cases := []struct {
		name     string
		urls     []string
		expected string
	}{
		{"nil falls back to tokens.json", nil, "tokens.json"},
		{"empty falls back to tokens.json", []string{}, "tokens.json"},
		{"production app host", []string{"https://app.quonfig.com"}, "tokens.json"},
		{"production primary host", []string{"https://primary.quonfig.com"}, "tokens.json"},
		{"plain quonfig.com", []string{"https://quonfig.com"}, "tokens.json"},
		{
			"staging app host",
			[]string{"https://app.quonfig-staging.com"},
			"tokens-quonfig-staging-com.json",
		},
		{
			"staging primary host",
			[]string{"https://primary.quonfig-staging.com"},
			"tokens-quonfig-staging-com.json",
		},
		{
			"multi-region uses first URL",
			[]string{"https://app.quonfig-staging.com", "https://app.quonfig.com"},
			"tokens-quonfig-staging-com.json",
		},
		{
			"unknown subdomain pattern preserved as-is",
			[]string{"https://quonfig-api-delivery-staging.fly.dev"},
			"tokens-quonfig-api-delivery-staging-fly-dev.json",
		},
		{"unparseable URL falls back", []string{"::not a url::"}, "tokens.json"},
		{"empty string URL falls back", []string{""}, "tokens.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tokenFilenameForAPIURLs(tc.urls)
			if got != tc.expected {
				t.Fatalf("tokenFilenameForAPIURLs(%v) = %q, want %q", tc.urls, got, tc.expected)
			}
		})
	}
}

func TestLoadQuonfigUserContext_StagingAPIURLReadsSuffixedFile(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFileNamed(t, home, "tokens-quonfig-staging-com.json", `{"userEmail":"jeff@quonfig.com"}`)
	// Plain tokens.json deliberately absent — staging dev should not fall back to prod file.

	cs := loadQuonfigUserContext([]string{"https://app.quonfig-staging.com"}, nil)
	got, ok := quonfigUserEmail(t, cs)
	if !ok || got != "jeff@quonfig.com" {
		t.Fatalf("expected jeff@quonfig.com from tokens-quonfig-staging-com.json, got %q ok=%v", got, ok)
	}
}

func TestLoadQuonfigUserContext_ProductionAPIURLReadsTokensJson(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"prod@example.com"}`)

	cs := loadQuonfigUserContext([]string{"https://app.quonfig.com"}, nil)
	got, ok := quonfigUserEmail(t, cs)
	if !ok || got != "prod@example.com" {
		t.Fatalf("expected prod@example.com from tokens.json, got %q ok=%v", got, ok)
	}
}

func TestLoadQuonfigUserContext_DefaultPrimaryQuonfigComReadsTokensJson(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"prod@example.com"}`)

	cs := loadQuonfigUserContext([]string{"https://primary.quonfig.com"}, nil)
	got, ok := quonfigUserEmail(t, cs)
	if !ok || got != "prod@example.com" {
		t.Fatalf("expected prod@example.com via default primary URL, got %q ok=%v", got, ok)
	}
}

func TestLoadQuonfigUserContext_NoAPIURLBackCompatTokensJson(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFile(t, home, `{"userEmail":"bob@foo.com"}`)

	cs := loadQuonfigUserContext(nil, nil)
	got, ok := quonfigUserEmail(t, cs)
	if !ok || got != "bob@foo.com" {
		t.Fatalf("expected back-compat read of tokens.json, got %q ok=%v", got, ok)
	}
}

func TestLoadQuonfigUserContext_StagingMissingFileNoOp(t *testing.T) {
	home := withTmpHome(t)
	// Only the prod file exists; the staging-suffixed file is absent.
	writeTokensFile(t, home, `{"userEmail":"prod@example.com"}`)

	cs := loadQuonfigUserContext([]string{"https://app.quonfig-staging.com"}, nil)
	if cs != nil {
		t.Fatalf("expected nil ContextSet when staging tokens file missing, got %+v", cs)
	}
}

func TestNewClient_StagingAPIURLReadsSuffixedTokensFile(t *testing.T) {
	home := withTmpHome(t)
	writeTokensFileNamed(t, home, "tokens-quonfig-staging-com.json", `{"userEmail":"jeff@quonfig.com"}`)
	t.Setenv("QUONFIG_DEV_CONTEXT", "")

	dir := emptyEnvelopeWorkspace(t)
	client, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithQuonfigUserContext(true),
		WithAPIURLs([]string{"https://app.quonfig-staging.com"}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	got, ok := quonfigUserEmail(t, client.opts.GlobalContext)
	if !ok || got != "jeff@quonfig.com" {
		t.Fatalf("expected staging email injection via NewClient, got %q ok=%v", got, ok)
	}
}
