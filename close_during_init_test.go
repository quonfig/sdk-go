package quonfig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// qfg-9dxb.10 (audit 2026-09-28b M1): Close() while the initial fetch is in
// flight must stop everything. Before the fix, startBackgroundWorkers ran
// after the fetch returned, did not check closeCh, and started a supervisor
// and fallback poller that kept polling with the SDK key forever.
func TestCloseDuringInitFetchStartsNoBackgroundWorkers(t *testing.T) {
	release := make(chan struct{})
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v2/configs") {
			http.Error(w, "stream unavailable", http.StatusServiceUnavailable)
			return
		}
		if hits.Add(1) == 1 {
			<-release
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"configs":[],"meta":{"version":"gen-1","environment":"Production","generation":1}}`))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(false),
		WithFallbackPoll(true, 20*time.Millisecond),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	waitFor(t, 3*time.Second, func() bool { return hits.Load() == 1 }, "init fetch never reached the server")
	client.Close()
	close(release)

	time.Sleep(300 * time.Millisecond)
	if got := hits.Load(); got != 1 {
		t.Fatalf("config fetches after Close = %d, want 1 (only the in-flight init fetch)", got)
	}
	if client.FallbackPollerActive() {
		t.Fatalf("fallback poller active after Close")
	}
}
