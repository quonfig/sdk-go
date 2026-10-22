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
	// reconnects), "connect_failed" (an attempt ended before its first frame
	// and the worker retries), "callback_panic" (a recovered panic in a user
	// or SDK callback on the SSE path). Allocated lazily under mu.
	mu       sync.Mutex
	restarts map[string]*atomic.Int64
}

// incRestart bumps the Layer 1 restart counter for reason.
func (m *workerMetrics) incRestart(reason string) {
	m.mu.Lock()
	v := m.restarts[reason]
	if v == nil {
		if m.restarts == nil {
			m.restarts = make(map[string]*atomic.Int64)
		}
		v = &atomic.Int64{}
		m.restarts[reason] = v
	}
	m.mu.Unlock()
	v.Add(1)
}

// restartTotal returns the Layer 1 restart count for reason, or the sum over
// every reason when reason is "".
func (m *workerMetrics) restartTotal(reason string) int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if reason != "" {
		if v := m.restarts[reason]; v != nil {
			return v.Load()
		}
		return 0
	}
	var sum int64
	for _, v := range m.restarts {
		sum += v.Load()
	}
	return sum
}
