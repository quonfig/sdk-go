package quonfig

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// tokenFilenameForAPIURLs picks the per-domain tokens file written by
// `qfg login`, mirroring cli/src/util/token-storage.ts:14. The CLI writes
// to `tokens.json` only when QUONFIG_DOMAIN=quonfig.com (the default);
// any other domain (e.g. quonfig-staging.com) is suffixed
// (`tokens-quonfig-staging-com.json`). The SDK derives the domain from the
// first configured API URL by stripping a leading "app." or "primary."
// subdomain. An empty list, an unparseable URL, or a host that resolves to
// quonfig.com falls back to the plain `tokens.json` name.
func tokenFilenameForAPIURLs(apiURLs []string) string {
	domain := deriveDomainFromAPIURLs(apiURLs)
	if domain == "" || domain == "quonfig.com" {
		return "tokens.json"
	}
	return "tokens-" + strings.ReplaceAll(domain, ".", "-") + ".json"
}

func deriveDomainFromAPIURLs(apiURLs []string) string {
	if len(apiURLs) == 0 || apiURLs[0] == "" {
		return ""
	}
	u, err := url.Parse(apiURLs[0])
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Hostname()
	for _, prefix := range []string{"app.", "primary."} {
		if strings.HasPrefix(host, prefix) {
			return host[len(prefix):]
		}
	}
	return host
}

// loadQuonfigUserContext reads the per-domain tokens file written by
// `qfg login` (~/.quonfig/tokens.json for production, or
// ~/.quonfig/tokens-<domain-with-dashes>.json for non-prod domains) and
// returns a ContextSet with quonfig-user.email populated when a userEmail
// is present. Returns nil when the file is missing, has no userEmail, or
// the home directory cannot be determined. A parse failure is logged once
// via slog.Warn and yields nil so SDK init can continue.
//
// The attribute is dev-only by construction: production servers do not run
// `qfg login` and therefore have no tokens file. Rules keyed on
// quonfig-user.email are dead code in prod.
func loadQuonfigUserContext(apiURLs []string, logger *slog.Logger) *ContextSet {
	if logger == nil {
		logger = slog.Default()
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	path := filepath.Join(home, ".quonfig", tokenFilenameForAPIURLs(apiURLs))

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		logger.Warn("quonfig dev-context: could not read tokens file; skipping injection",
			"path", path, "err", err.Error())
		return nil
	}

	var parsed struct {
		UserEmail string `json:"userEmail"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		logger.Warn("quonfig dev-context: could not parse tokens file; skipping injection",
			"path", path, "err", err.Error())
		return nil
	}

	if parsed.UserEmail == "" {
		return nil
	}

	return NewContextSet().WithNamedContextValues("quonfig-user", map[string]interface{}{
		"email": parsed.UserEmail,
	})
}
