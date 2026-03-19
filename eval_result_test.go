package quonfig

import "testing"

func TestEvalReason_String(t *testing.T) {
	tests := []struct {
		reason   EvalReason
		expected string
	}{
		{ReasonUnknown, "UNKNOWN"},
		{ReasonStatic, "STATIC"},
		{ReasonTargetingMatch, "TARGETING_MATCH"},
		{ReasonSplit, "SPLIT"},
		{ReasonDefault, "DEFAULT"},
		{ReasonError, "ERROR"},
		{EvalReason(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.reason.String(); got != tt.expected {
			t.Errorf("EvalReason(%d).String() = %q, want %q", tt.reason, got, tt.expected)
		}
	}
}
