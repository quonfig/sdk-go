package fixtures

import (
	"errors"
	"testing"

	quonfig "github.com/quonfig/sdk-go"
)

// Malformed-duration contract through the PUBLIC getter (qfg-2agi.13; plan
// project/plans/2026-10-01-duration-validity.md, decision 3). Go has no
// default-arg getter, so the (value, ok, err) form is both the no-default
// and the raise variant: it returns the absent value (0, ok=false) and the
// public coercion error, for stored and ENV_VAR-provided values alike.
func TestGetDurationValue_MalformedReturnsAbsentAndCoercionError(t *testing.T) {
	cases := []struct {
		key, envVar, envValue string
	}{
		{key: "test.duration.malformed.30s"},
		{key: "test.duration.malformed.PT0.5H"},
		{key: "test.duration.malformed.P1DT"},
		{key: "test.duration.malformed.garbage"},
		{key: "test.duration.malformed.empty"},
		{key: "provided.duration.malformed.30s", envVar: "QUONFIG_ITD_DURATION_30S", envValue: "30s"},
		{key: "provided.duration.malformed.PT0.5H", envVar: "QUONFIG_ITD_DURATION_PT0_5H", envValue: "PT0.5H"},
		{key: "provided.duration.malformed.P1DT", envVar: "QUONFIG_ITD_DURATION_P1DT", envValue: "P1DT"},
		{key: "provided.duration.malformed.garbage", envVar: "QUONFIG_ITD_DURATION_GARBAGE", envValue: "garbage"},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if c.envVar != "" {
				t.Setenv(c.envVar, c.envValue)
			}
			got, ok, err := mustPublicClient(t).GetDurationValue(c.key, nil)
			if got != 0 || ok {
				t.Errorf("GetDurationValue(%q) = (%v, ok=%v), want (0, ok=false)", c.key, got, ok)
			}
			if !errors.Is(err, quonfig.ErrUnableToCoerce) {
				t.Errorf("GetDurationValue(%q) err = %v, want errors.Is ErrUnableToCoerce", c.key, err)
			}
		})
	}
}
