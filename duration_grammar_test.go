package quonfig

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// grammarFixture mirrors integration-test-data/tests/duration/grammar.yaml,
// the one shared definition of which ISO-8601 duration strings Quonfig
// accepts and what they mean in milliseconds (qfg-2agi.29).
type grammarFixture struct {
	Valid []struct {
		Value  string `yaml:"value"`
		Millis int64  `yaml:"millis"`
	} `yaml:"valid"`
	Invalid []string `yaml:"invalid"`
}

func loadGrammarFixture(t *testing.T) grammarFixture {
	t.Helper()
	path := filepath.Join("..", "integration-test-data", "tests", "duration", "grammar.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading shared duration grammar fixture %s: %v", path, err)
	}
	var f grammarFixture
	if err := yaml.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(f.Valid) == 0 || len(f.Invalid) == 0 {
		t.Fatalf("%s: empty valid/invalid list", path)
	}
	return f
}

func TestParseISO8601Duration_GrammarFixtureValid(t *testing.T) {
	for _, c := range loadGrammarFixture(t).Valid {
		got, err := ParseISO8601Duration(c.Value)
		if err != nil {
			t.Errorf("ParseISO8601Duration(%q) error: %v, want %dms", c.Value, err, c.Millis)
			continue
		}
		if want := time.Duration(c.Millis) * time.Millisecond; got != want {
			t.Errorf("ParseISO8601Duration(%q) = %v (%dns), want exactly %dms", c.Value, got, got.Nanoseconds(), c.Millis)
		}
	}
}

func TestParseISO8601Duration_GrammarFixtureInvalid(t *testing.T) {
	for _, s := range loadGrammarFixture(t).Invalid {
		if got, err := ParseISO8601Duration(s); err == nil {
			t.Errorf("ParseISO8601Duration(%q) = %v, want an error", s, got)
		}
	}
}
