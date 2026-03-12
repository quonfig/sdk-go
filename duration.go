package quonfig

import (
	"fmt"
	"math"
	"time"
)

// ParseISO8601Duration parses an ISO 8601 duration string and returns a time.Duration.
// Supports: P[n]Y[n]M[n]W[n]DT[n]H[n]M[n]S
// Examples: PT0.2S, PT90S, PT1.5M, PT0.5H, P1DT6H2M1.5S
func ParseISO8601Duration(s string) (time.Duration, error) {
	if len(s) < 2 || s[0] != 'P' {
		return 0, fmt.Errorf("invalid ISO 8601 duration: %s", s)
	}

	var totalMillis float64
	i := 1 // skip 'P'
	inTimePart := false

	for i < len(s) {
		if s[i] == 'T' {
			inTimePart = true
			i++
			continue
		}

		// Parse the number
		start := i
		for i < len(s) && (s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
			i++
		}
		if i >= len(s) {
			return 0, fmt.Errorf("invalid ISO 8601 duration: unexpected end: %s", s)
		}

		numStr := s[start:i]
		var num float64
		if _, err := fmt.Sscanf(numStr, "%f", &num); err != nil {
			return 0, fmt.Errorf("invalid number in duration %q: %w", numStr, err)
		}

		unit := s[i]
		i++

		if inTimePart {
			switch unit {
			case 'H':
				totalMillis += num * 3600000
			case 'M':
				totalMillis += num * 60000
			case 'S':
				totalMillis += num * 1000
			default:
				return 0, fmt.Errorf("unknown time unit %c in duration %s", unit, s)
			}
		} else {
			switch unit {
			case 'Y':
				totalMillis += num * 365.25 * 86400000
			case 'M':
				totalMillis += num * 30 * 86400000
			case 'W':
				totalMillis += num * 7 * 86400000
			case 'D':
				totalMillis += num * 86400000
			default:
				return 0, fmt.Errorf("unknown date unit %c in duration %s", unit, s)
			}
		}
	}

	return time.Duration(int64(math.Round(totalMillis))) * time.Millisecond, nil
}
