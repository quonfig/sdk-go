package quonfig

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// durationPattern is the shared Quonfig duration grammar
// (integration-test-data/tests/duration/grammar.yaml): days, hours, minutes
// and seconds in that order, each at most once, a fraction only on seconds
// (1-9 digits). ASCII digits only and a full-string match (\z), so trailing
// newlines and non-ASCII digits are rejected.
var durationPattern = regexp.MustCompile(`^P(?:([0-9]+)D)?(?:(T)(?:([0-9]+)H)?(?:([0-9]+)M)?(?:([0-9]+)(?:\.([0-9]{1,9}))?S)?)?\z`)

// maxDurationSeconds is the magnitude ceiling, P36500D.
const maxDurationSeconds = 36500 * 86400

// ParseISO8601Duration parses a Quonfig ISO 8601 duration string and returns
// a time.Duration with millisecond precision.
//
// Grammar: P[nD][T[nH][nM][n[.fff]S]] with at least one component, no
// dangling T, a fraction only on S (at most 9 digits), and a magnitude of at
// most P36500D. Years, months and weeks are not supported. Fractional seconds
// are converted with exact decimal arithmetic and rounded half up to an
// integer millisecond count. Examples: PT0.2S, PT90S, PT30M, P1DT6H2M1.5S.
func ParseISO8601Duration(s string) (time.Duration, error) {
	m := durationPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("invalid ISO 8601 duration: %q", s)
	}
	days, tSep, hours, minutes, secs, frac := m[1], m[2], m[3], m[4], m[5], m[6]
	if tSep != "" && hours == "" && minutes == "" && secs == "" {
		return 0, fmt.Errorf("invalid ISO 8601 duration: %q has a T with no time component", s)
	}
	if days == "" && tSep == "" {
		return 0, fmt.Errorf("invalid ISO 8601 duration: %q has no component", s)
	}

	var totalSeconds uint64
	for _, part := range []struct {
		digits string
		unit   uint64
	}{{days, 86400}, {hours, 3600}, {minutes, 60}, {secs, 1}} {
		if part.digits == "" {
			continue
		}
		n, err := strconv.ParseUint(part.digits, 10, 64)
		// Bound each component before multiplying so nothing can overflow.
		if err != nil || n > maxDurationSeconds/part.unit {
			return 0, fmt.Errorf("invalid ISO 8601 duration: %q exceeds P36500D", s)
		}
		totalSeconds += n * part.unit
	}

	var fracNanos uint64
	if frac != "" {
		fracNanos, _ = strconv.ParseUint(frac+strings.Repeat("0", 9-len(frac)), 10, 64)
	}
	if totalSeconds > maxDurationSeconds || (totalSeconds == maxDurationSeconds && fracNanos > 0) {
		return 0, fmt.Errorf("invalid ISO 8601 duration: %q exceeds P36500D", s)
	}

	// Round half up to whole milliseconds.
	millis := totalSeconds*1000 + (fracNanos+500_000)/1_000_000
	return time.Duration(millis) * time.Millisecond, nil
}
