package telemetry

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Telemetry transport policy (qfg-y8je.6; policy P1-P10 in
// project/plans/2026-09-24-sdk-telemetry-transport-policy.md, contract tests
// in integration-test-data/chaos/telemetry-transport-contract.md). Mirrors the
// sdk-node reference implementation (src/telemetry/transportQueue.ts).
//
// transportQueue owns the retained queue of serialized batches, the send gate
// (30s floor after a failure + Retry-After), disable-on-auth and the P7
// logging episodes. It knows nothing about aggregators or payload shape: it
// stores and resends opaque bytes. The drain loop lives on the Submitter.

// Shipped defaults for the server SDK class.
const (
	DefaultSyncInterval           = 60 * time.Second
	DefaultTimeout                = 15 * time.Second
	DefaultConnectTimeout         = 5 * time.Second
	DefaultMaxRetainedBatches     = 5
	DefaultMaxRetainedBytes       = 2 * 1024 * 1024
	DefaultMaxRetainedAge         = 5 * time.Minute
	DefaultMaxEvaluationSummaries = 10_000
	DefaultMaxContextShapeFields  = 10_000
	DefaultMaxExampleContexts     = 10_000
)

const (
	// ResendFloor: no send sooner than this after a failed POST (P4).
	ResendFloor = 30 * time.Second
	// RetryAfterCap: Retry-After is honored up to this (P4).
	RetryAfterCap = 10 * time.Minute
	// DropWarnInterval: at most one drop WARN per this interval (P7).
	DropWarnInterval = 10 * time.Minute
	// ShutdownFlushDeadline: Stop() gives the live window one POST with this
	// deadline (P8).
	ShutdownFlushDeadline = 5 * time.Second
)

// statusClass is the P3 classification of an HTTP status.
type statusClass int

const (
	statusOK statusClass = iota
	statusRetryable
	statusAuth
	statusRejected
)

// classifyStatus: 2xx -> ok; 401, 403, 404 -> auth; 408, 429, 5xx ->
// retryable; every other status (other 4xx, 3xx, 1xx) -> rejected (P3).
func classifyStatus(status int) statusClass {
	switch {
	case status >= 200 && status < 300:
		return statusOK
	case status == 401 || status == 403 || status == 404:
		return statusAuth
	case status == 408 || status == 429 || (status >= 500 && status < 600):
		return statusRetryable
	default:
		return statusRejected
	}
}

// parseRetryAfter parses a Retry-After header into a wait: delta-seconds, or
// an HTTP-date relative to now (past dates -> 0). ok is false when absent or
// unparseable. The result is clamped to RetryAfterCap.
func parseRetryAfter(header string, now time.Time) (time.Duration, bool) {
	v := strings.TrimSpace(header)
	if v == "" {
		return 0, false
	}
	var d time.Duration
	if isDigits(v) {
		secs, err := strconv.ParseInt(v, 10, 64)
		if err != nil || secs > int64(RetryAfterCap/time.Second) {
			// Overflowing or huge values clamp.
			return RetryAfterCap, true
		}
		d = time.Duration(secs) * time.Second
	} else {
		at, err := http.ParseTime(v)
		if err != nil {
			return 0, false
		}
		d = at.Sub(now)
		if d < 0 {
			d = 0
		}
	}
	if d > RetryAfterCap {
		d = RetryAfterCap
	}
	return d, true
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

type retainedBatch struct {
	body      []byte
	createdAt time.Time
	oversize  bool
}

type transportQueue struct {
	logger             *slog.Logger
	clock              Clock
	telemetryURL       string
	maxRetainedBatches int
	maxRetainedBytes   int
	maxRetainedAge     time.Duration

	mu              sync.Mutex
	queue           []*retainedBatch // head = oldest
	lastFailureAt   time.Time        // zero = never
	retryAfterUntil time.Time
	disabled        atomic.Bool // read on the evaluation hot path

	// Outage episode (P7).
	failuresSinceSuccess int
	firstFailureAt       time.Time
	lastResult           string
	lastDropWarnAt       time.Time // zero = no WARN this episode
	dropsSinceWarn       int
	dropsThisOutage      int

	// Rejected-batch (other 4xx) cadence.
	lastRejectErrorAt time.Time
	rejectsSinceError int
}

func (q *transportQueue) isDisabled() bool {
	return q.disabled.Load()
}

// counts returns the retained-queue depth (every queued batch, including a
// not-yet-sent oversize one) and total bytes. Caller holds mu.
func (q *transportQueue) countsLocked() (int, int) {
	n := 0
	for _, b := range q.queue {
		n += len(b.body)
	}
	return len(q.queue), n
}

func (q *transportQueue) counts() (int, int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.countsLocked()
}

// expire discards batches older than the max age (strictly greater). Tick
// step 2.
func (q *transportQueue) expire() {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.clock.Now()
	for len(q.queue) > 0 && now.Sub(q.queue[0].createdAt) > q.maxRetainedAge {
		q.queue = q.queue[1:]
		q.recordDropLocked(fmt.Sprintf("batch older than %d min", int(math.Round(q.maxRetainedAge.Minutes()))))
	}
}

// sendAllowed: the 30s floor after a failure and any Retry-After have both
// elapsed. Tick step 3.
func (q *transportQueue) sendAllowed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.clock.Now()
	if !q.lastFailureAt.IsZero() && now.Before(q.lastFailureAt.Add(ResendFloor)) {
		return false
	}
	return !now.Before(q.retryAfterUntil)
}

// append adds a serialized window at the tail and enforces the caps (drop
// oldest). An oversize batch is never counted against, or evicted by, the
// caps; it is sent once and then dropped. Tick step 4.
func (q *transportQueue) append(body []byte) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queue = append(q.queue, &retainedBatch{
		body:      body,
		createdAt: q.clock.Now(),
		oversize:  len(body) > q.maxRetainedBytes,
	})
	count, bytes := 0, 0
	for _, b := range q.queue {
		if !b.oversize {
			count++
			bytes += len(b.body)
		}
	}
	for count > q.maxRetainedBatches || bytes > q.maxRetainedBytes {
		i := -1
		for j, b := range q.queue {
			if !b.oversize {
				i = j
				break
			}
		}
		if i < 0 {
			break
		}
		evicted := q.queue[i]
		q.queue = append(q.queue[:i:i], q.queue[i+1:]...)
		count--
		bytes -= len(evicted.body)
		q.recordDropLocked("retained queue full")
	}
}

// head returns the oldest queued batch, or nil.
func (q *transportQueue) head() *retainedBatch {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.disabled.Load() || len(q.queue) == 0 {
		return nil
	}
	return q.queue[0]
}

func (q *transportQueue) removeLocked(b *retainedBatch) {
	for i, x := range q.queue {
		if x == b {
			q.queue = append(q.queue[:i:i], q.queue[i+1:]...)
			return
		}
	}
}

// onSuccess removes the sent batch and logs recovery once per outage.
func (q *transportQueue) onSuccess(b *retainedBatch) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.removeLocked(b)
	if q.failuresSinceSuccess == 0 {
		return
	}
	now := q.clock.Now()
	seconds := int(math.Round(now.Sub(q.firstFailureAt).Seconds()))
	q.logger.Info(fmt.Sprintf(
		"quonfig: Telemetry recovered: POST succeeded after %d failed attempt(s) over %ds; %d batch(es) were dropped.",
		q.failuresSinceSuccess, seconds, q.dropsThisOutage))
	q.failuresSinceSuccess = 0
	q.firstFailureAt = time.Time{}
	q.dropsThisOutage = 0
	q.lastDropWarnAt = time.Time{}
	q.dropsSinceWarn = 0
}

// onRetryableFailure keeps the batch (unless oversize), records the failure
// time and any Retry-After, and logs at DEBUG.
func (q *transportQueue) onRetryableFailure(b *retainedBatch, result, retryAfter string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := q.clock.Now()
	q.failuresSinceSuccess++
	if q.firstFailureAt.IsZero() {
		q.firstFailureAt = now
	}
	q.lastFailureAt = now
	q.lastResult = result
	if wait, ok := parseRetryAfter(retryAfter, now); ok {
		q.retryAfterUntil = now.Add(wait)
	}
	next := q.lastFailureAt.Add(ResendFloor)
	if q.retryAfterUntil.After(next) {
		next = q.retryAfterUntil
	}
	count, bytes := q.countsLocked()
	q.logger.Debug(fmt.Sprintf(
		"quonfig: Telemetry POST failed (%s); %d batch(es) / %d bytes retained, next send in >= %ds",
		result, count, bytes, int(math.Ceil(next.Sub(now).Seconds()))))
	if b.oversize {
		q.removeLocked(b)
		q.recordDropLocked("batch larger than the byte cap")
	}
}

// disable handles 401/403/404: one ERROR, drop the queue, telemetry off for
// the process.
func (q *transportQueue) disable(status int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	hint := "the SDK key was rejected"
	if status == 404 {
		hint = "wrong telemetry URL"
	}
	q.logger.Error(fmt.Sprintf(
		"quonfig: Telemetry disabled for this process: %s answered %d (%s). Flag evaluation is unaffected.",
		q.telemetryURL, status, hint))
	q.queue = nil
	q.disabled.Store(true)
}

// onRejected drops a batch the server rejected (other 4xx). ERROR at most
// once per 10 min, DEBUG otherwise. Not a failure: no floor.
func (q *transportQueue) onRejected(b *retainedBatch, status int, bodySnippet string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.removeLocked(b)
	now := q.clock.Now()
	if q.lastRejectErrorAt.IsZero() || now.Sub(q.lastRejectErrorAt) >= DropWarnInterval {
		more := ""
		if q.rejectsSinceError > 0 {
			more = fmt.Sprintf(", %d more since the last report", q.rejectsSinceError)
		}
		q.logger.Error(fmt.Sprintf(
			"quonfig: Telemetry batch rejected with %d and dropped (%d bytes%s): %s. This is likely an SDK bug; please report it.",
			status, len(b.body), more, bodySnippet))
		q.lastRejectErrorAt = now
		q.rejectsSinceError = 0
		return
	}
	q.rejectsSinceError++
	q.logger.Debug(fmt.Sprintf("quonfig: Telemetry batch rejected with %d and dropped (%d bytes)", status, len(b.body)))
}

// dropOversize drops any oversize batch still queued at the end of a drain:
// oversize batches are never carried across ticks.
func (q *transportQueue) dropOversize() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := len(q.queue) - 1; i >= 0; i-- {
		if q.queue[i].oversize {
			q.queue = append(q.queue[:i:i], q.queue[i+1:]...)
			q.recordDropLocked("batch larger than the byte cap")
		}
	}
}

// recordDropLocked implements the P7 drop cadence: first drop of an outage
// WARN, later drops DEBUG, a summary WARN at most once per 10 min. The
// episode resets only on recovery. Caller holds mu.
func (q *transportQueue) recordDropLocked(reason string) {
	now := q.clock.Now()
	q.dropsSinceWarn++
	q.dropsThisOutage++
	lastResult := q.lastResult
	if lastResult == "" {
		lastResult = "none"
	}
	count, bytes := q.countsLocked()
	switch {
	case q.lastDropWarnAt.IsZero():
		q.logger.Warn(fmt.Sprintf(
			"quonfig: Telemetry is dropping data: %s (last POST result: %s). %d batch(es) dropped so far; retained queue %d/%d batches, %d bytes. Flag evaluation is unaffected; further drops log at debug with a summary every 10 min.",
			reason, lastResult, q.dropsThisOutage, count, q.maxRetainedBatches, bytes))
		q.lastDropWarnAt = now
		q.dropsSinceWarn = 0
	case now.Sub(q.lastDropWarnAt) >= DropWarnInterval:
		minutes := int(math.Round(now.Sub(q.lastDropWarnAt).Minutes()))
		q.logger.Warn(fmt.Sprintf(
			"quonfig: Telemetry still dropping data: %d batch(es) dropped in the last %d min (last POST result: %s); retained queue %d batches, %d bytes.",
			q.dropsSinceWarn, minutes, lastResult, count, bytes))
		q.lastDropWarnAt = now
		q.dropsSinceWarn = 0
	default:
		q.logger.Debug(fmt.Sprintf("quonfig: Telemetry dropped a batch: %s; %d since the last warning", reason, q.dropsSinceWarn))
	}
}
