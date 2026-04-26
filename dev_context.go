package quonfig

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// loadQuonfigUserContext reads ~/.quonfig/tokens.json (written by `qfg login`)
// and returns a ContextSet with quonfig-user.email populated when a userEmail
// is present. Returns nil when the file is missing, has no userEmail, or the
// home directory cannot be determined. A parse failure is logged once via
// slog.Warn and yields nil so SDK init can continue.
//
// The attribute is dev-only by construction: production servers do not run
// `qfg login` and therefore have no tokens file. Rules keyed on
// quonfig-user.email are dead code in prod.
func loadQuonfigUserContext() *ContextSet {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil
	}
	path := filepath.Join(home, ".quonfig", "tokens.json")

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		slog.Warn("quonfig dev-context: could not read tokens file; skipping injection",
			"path", path, "err", err.Error())
		return nil
	}

	var parsed struct {
		UserEmail string `json:"userEmail"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		slog.Warn("quonfig dev-context: could not parse tokens file; skipping injection",
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
