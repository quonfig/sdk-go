package telemetry

import (
	"context"
	"time"
)

// Clock is the time source for the telemetry transport: the tick timer, the
// per-POST deadline, the 30s resend floor, Retry-After and retained-batch age
// all read it. Production uses the wall clock; the contract tests inject a
// manual clock so they never sleep for real (qfg-y8je.6).
type Clock interface {
	Now() time.Time
	// AfterFunc runs f on its own goroutine once d has elapsed, like
	// time.AfterFunc.
	AfterFunc(d time.Duration, f func()) Timer
	// WithTimeout derives a context that is canceled with cause
	// context.DeadlineExceeded once d has elapsed on this clock.
	WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc)
}

// Timer is a pending AfterFunc callback.
type Timer interface {
	Stop() bool
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) AfterFunc(d time.Duration, f func()) Timer { return time.AfterFunc(d, f) }

func (realClock) WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

// RealClock is the wall clock.
var RealClock Clock = realClock{}
