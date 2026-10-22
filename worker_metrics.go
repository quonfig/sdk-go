package quonfig

import (
	"sync"
	"sync/atomic"
)

// workerMetrics holds the SDK's Layer 1 (SSE) resilience counters for one
// Client. It is internal and test-only: nothing exports it, and it changes no
// SDK behavior. The in-package chaos harness reads it so every metric a
// scenario asserts on comes from the SDK itself rather than from the probe's
// own bookkeeping (qfg-6vno.1). It lives on the Client, not the sseClient,
// because the SSE client is replaced when the stream restarts after an auth
// failure and the counts must survive that.
//
// Layer 2 (fallback poller) restarts are counted by the supervisor
// (supervisor.WorkerRestartTotal).
type workerMetrics struct {
	// sseConnectAttempts backs quonfig_sse_connect_attempts_total: one per
	// SSE connection attempt (connectOnce), whatever its outcome.
	sseConnectAttempts atomic.Int64

	// restarts backs quonfig_sdk_worker_restart_total{layer="1",reason=...}.
	// Reasons: "stream_ended" (a live stream ended and the worker
	// reconnects) and "connect_failed" (an attempt ended before its first
	// frame and the worker retries). A callback panic is never a restart
	// (contract Test 5, qfg-6vno.26). Allocated lazily under mu.
	mu       sync.Mutex
	restarts map[string]*atomic.Int64

	// callbackErrors backs quonfig_sdk_callback_errors_total{callback=...}:
	// one per user callback invocation that panicked and was recovered.
	// Callbacks: "on_config_update", "on_connection_state_change", and
	// "on_envelope" (the SDK's own SSE envelope hook, a last-line guard).
	// Allocated lazily under mu.
	callbackErrors map[string]*atomic.Int64
}

// incRestart bumps the Layer 1 restart counter for reason.
func (m *workerMetrics) incRestart(reason string) {
	m.inc(&m.restarts, reason)
}

// restartTotal returns the Layer 1 restart count for reason, or the sum over
// every reason when reason is "".
func (m *workerMetrics) restartTotal(reason string) int64 {
	return m.total(&m.restarts, reason)
}

// incCallbackError bumps quonfig_sdk_callback_errors_total for callback.
func (m *workerMetrics) incCallbackError(callback string) {
	m.inc(&m.callbackErrors, callback)
}

// callbackErrorTotal returns the callback error count for callback, or the
// sum over every callback when callback is "".
func (m *workerMetrics) callbackErrorTotal(callback string) int64 {
	return m.total(&m.callbackErrors, callback)
}

// inc and total take a pointer to the map field so the field itself is only
// read and written under mu.
func (m *workerMetrics) inc(counters *map[string]*atomic.Int64, label string) {
	m.mu.Lock()
	v := (*counters)[label]
	if v == nil {
		if *counters == nil {
			*counters = make(map[string]*atomic.Int64)
		}
		v = &atomic.Int64{}
		(*counters)[label] = v
	}
	m.mu.Unlock()
	v.Add(1)
}

func (m *workerMetrics) total(counters *map[string]*atomic.Int64, label string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if label != "" {
		if v := (*counters)[label]; v != nil {
			return v.Load()
		}
		return 0
	}
	var sum int64
	for _, v := range *counters {
		sum += v.Load()
	}
	return sum
}
