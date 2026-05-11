//go:build chaos

package quonfig

import "time"

// withTestSSEReadTimeout overrides the SSE client's per-read idle deadline.
// Production defaults to 90s; chaos scenarios use 1–5s so they complete in
// seconds instead of minutes. Gated by `//go:build chaos` because the only
// caller is chaos_test.go — without the tag, staticcheck (U1000) flags it
// as unused. The corresponding field Options.testSSEReadTimeout stays in
// options.go since it is read from production code (quonfig.go).
func withTestSSEReadTimeout(d time.Duration) Option {
	return func(o *Options) error {
		o.testSSEReadTimeout = d
		return nil
	}
}
