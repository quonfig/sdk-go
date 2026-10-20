package quonfig

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// qfg-9dxb.10 (audit 2026-09-28b M1): Close() must stop everything that
// retries with the SDK key. NewClient now blocks through init (qfg-9p4i.7), so
// the reachable form of "Close while the init fetch is unresolved" is a
// continue client still retrying in the background: Close must stop the init
// retry loop and the fallback poller.
func TestCloseDuringInitFetchStartsNoBackgroundWorkers(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v2/configs") {
			hits.Add(1)
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(false),
		WithFallbackPoll(true, 20*time.Millisecond),
		WithAllTelemetryDisabled(),
		WithInitTimeout(50*time.Millisecond),
		WithOnInitFailure(InitFailureContinue),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	waitFor(t, 3*time.Second, func() bool { return hits.Load() >= 2 }, "background retry never reached the server")
	client.Close()
	time.Sleep(50 * time.Millisecond) // let an in-flight request settle
	after := hits.Load()

	time.Sleep(300 * time.Millisecond)
	if got := hits.Load(); got != after {
		t.Fatalf("config fetches after Close went %d -> %d, want no new fetches", after, got)
	}
}
