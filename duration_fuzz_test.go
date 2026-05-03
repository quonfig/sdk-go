package quonfig

import "testing"

// FuzzParseISO8601Duration looks for inputs that panic or hang the parser.
// The parser is fed untrusted strings (config values), so a panic here would
// take down a customer's process.
func FuzzParseISO8601Duration(f *testing.F) {
	seeds := []string{
		"PT0S",
		"PT0.2S",
		"PT90S",
		"PT1.5M",
		"PT0.5H",
		"P1DT6H2M1.5S",
		"P1Y2M3W4DT5H6M7.89S",
		"",
		"P",
		"PT",
		"X",
		"P1",
		"PT.S",
		"PT..S",
		"P1.2.3DT",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		// We don't care about the result — only that the parser does not
		// panic, hang, or otherwise misbehave on arbitrary input.
		_, _ = ParseISO8601Duration(s)
	})
}
