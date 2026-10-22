//go:build chaos

package quonfig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The chaos probe must report what the SDK itself counted, never its own
// bookkeeping (qfg-6vno.1). Each test forces the real condition and checks
// the probe sees it. They need no toxiproxy, so they run in any
// `go test -tags chaos` invocation.

// worker_restart_total{layer="2"} must come from the supervisor's counter. A
// probe that hard-codes 0 lets scenario 11's "layer 2 == 0" pass against a
// constant even while the fallback poller is crash-looping.
func TestChaosProbeLayer2RestartsComeFromSupervisor(t *testing.T) {
	c := &Client{sup: newSupervisor(supervisorConfig{})}
	c.sup.incRestartTotal("2")
	c.sup.incRestartTotal("2")

	p := newChaosProbe()
	p.setClient(c)
	got, known := p.sdkMetric("quonfig_sdk_worker_restart_total", map[string]string{"layer": "2"})
	if !known || got != 2 {
		t.Fatalf("layer=2 restarts = %v (known=%v), want 2 from the supervisor counter", got, known)
	}
	if r := evaluate("client.sdkMetric('quonfig_sdk_worker_restart_total', layer='2') == 0", &evalCtx{probe: p}); r.verdict != verdictFail {
		t.Fatalf("scenario 11 row 'layer 2 == 0' = %s (%s) with 2 real layer-2 restarts, want fail", r.verdict, r.why)
	}
}

// failingStreamClient starts a real Client whose config fetch succeeds and
// whose SSE endpoint answers every attempt with 503, so no connection ever
// delivers a frame. Reconnect sleeps are shrunk to 1ms. It returns once the
// server has seen at least `attempts` SSE requests.
func failingStreamClient(t *testing.T, p *chaosProbe, attempts int32) *Client {
	t.Helper()
	var sseHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v2/configs") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(firstFrameEnvelope(1, "v1")))
			return
		}
		sseHits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	opts := append(chaosProbeClientOptions(p),
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(true),
		withTestStreamURLOverride(srv.URL+"/api/v2/sse/config"),
		WithFallbackPoll(false, 0),
		withTestWaitHook(func(kind string, d time.Duration) time.Duration {
			if kind == "sse" {
				return time.Millisecond
			}
			return d
		}),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
	)
	c, err := NewClient(opts...)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Close)
	p.setClient(c)

	deadline := time.Now().Add(5 * time.Second)
	for sseHits.Load() < attempts {
		if time.Now().After(deadline) {
			t.Fatalf("server saw %d SSE attempts, want >= %d", sseHits.Load(), attempts)
		}
		time.Sleep(5 * time.Millisecond)
	}
	return c
}

// quonfig_sse_connect_attempts_total must count connection attempts, not
// connected edges: under the first-frame rule a failed-attempt storm has no
// connected edges at all, so scenario 09's "< 100" could never see it.
func TestChaosProbeConnectAttemptsCountFailedAttempts(t *testing.T) {
	p := newChaosProbe()
	failingStreamClient(t, p, 5)
	got, known := p.sdkMetric("quonfig_sse_connect_attempts_total", nil)
	if !known || got < 5 {
		t.Fatalf("connect attempts = %v (known=%v) after >= 5 failed attempts, want >= 5", got, known)
	}
}

// worker_restart_total{layer="1"} must come from the SDK's SSE worker: every
// reconnect after a failed attempt is a Layer 1 restart, even though no
// connection ever reached "connected".
func TestChaosProbeLayer1RestartsComeFromSSEWorker(t *testing.T) {
	p := newChaosProbe()
	failingStreamClient(t, p, 5)
	got, known := p.sdkMetric("quonfig_sdk_worker_restart_total", map[string]string{"layer": "1"})
	if !known || got < 4 {
		t.Fatalf("layer=1 restarts = %v (known=%v) after >= 5 failed attempts, want >= 4", got, known)
	}
}

// The probe may only report a connectionState() value the supervisor
// contract allows.
func TestChaosProbeConnectionStateIsContractVocabulary(t *testing.T) {
	p := newChaosProbe()
	failingStreamClient(t, p, 2)
	switch s := p.connectionState(); s {
	case "initializing", "connected", "disconnected", "falling_back":
	default:
		t.Fatalf("connectionState() = %q, not a supervisor-contract value", s)
	}
}
