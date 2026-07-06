package quonfig_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	quonfig "github.com/quonfig/sdk-go"
)

// Failover telemetry emission (qfg-41nh.18). These drive the real client paths
// that must emit the new failover signals and assert they land on the wire in
// the exact JSON shape api-telemetry parses. The struct below intentionally
// mirrors the api-telemetry Zod schema field names (verify-shape-before-writing)
// rather than reusing the internal types, so a rename would fail this test.
type capturedFailover struct {
	HedgeFired            int64 `json:"hedgeFired"`
	GuardRejected         int64 `json:"guardRejected"`
	ResolvedFromPrimary   int64 `json:"resolvedFromPrimary"`
	ResolvedFromSecondary int64 `json:"resolvedFromSecondary"`
	ResolvedFromLkg       int64 `json:"resolvedFromLkg"`
}

// telemetryCapture is an httptest server that accumulates the failover counters
// across every telemetry POST it receives.
type telemetryCapture struct {
	server *httptest.Server
	mu     sync.Mutex
	total  capturedFailover
	sawAny bool
}

func newTelemetryCapture(t *testing.T) *telemetryCapture {
	t.Helper()
	c := &telemetryCapture{}
	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Events []struct {
				Failover *capturedFailover `json:"failover"`
			} `json:"events"`
		}
		_ = json.Unmarshal(body, &payload)
		c.mu.Lock()
		for _, ev := range payload.Events {
			if ev.Failover != nil {
				c.sawAny = true
				c.total.HedgeFired += ev.Failover.HedgeFired
				c.total.GuardRejected += ev.Failover.GuardRejected
				c.total.ResolvedFromPrimary += ev.Failover.ResolvedFromPrimary
				c.total.ResolvedFromSecondary += ev.Failover.ResolvedFromSecondary
				c.total.ResolvedFromLkg += ev.Failover.ResolvedFromLkg
			}
		}
		c.mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(c.server.Close)
	return c
}

func (c *telemetryCapture) get() capturedFailover {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}

// erroringUpstream always answers 500 (a fast primary error that triggers the
// hedge immediately).
func erroringUpstream(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// mutableUpstream serves whatever generation `gen` currently holds, honoring
// If-None-Match so an unchanged fetch is a 304.
func mutableUpstream(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var gen atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g := gen.Load()
		etag := fmt.Sprintf(`"gen-%d"`, g)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(hedgeEnvelopeJSON(int(g))))
	}))
	t.Cleanup(srv.Close)
	return srv, &gen
}

// TestFailoverTelemetry_HedgeAndSecondary: the primary errors fast, so the
// hedge fires the secondary, which serves the config. The client must emit
// hedgeFired>=1 and resolvedFromSecondary>=1.
func TestFailoverTelemetry_HedgeAndSecondary(t *testing.T) {
	cap := newTelemetryCapture(t)
	primary, _ := erroringUpstream(t)
	secondary, _ := hedgeUpstream(t, 5, 0)

	client, err := quonfig.NewClient(
		quonfig.WithSdkKey("test-backend-key"),
		quonfig.WithAPIURLs([]string{primary.URL, secondary.URL}),
		quonfig.WithTelemetryURL(cap.server.URL),
		quonfig.WithTelemetrySyncInterval(time.Minute),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithInitTimeout(8*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	awaitReady(t, client)
	if got := client.ResolvedFrom(); got != "secondary" {
		t.Fatalf("ResolvedFrom = %q, want secondary", got)
	}
	client.Close() // flushes telemetry

	got := cap.get()
	if got.HedgeFired < 1 {
		t.Errorf("HedgeFired = %d, want >= 1", got.HedgeFired)
	}
	if got.ResolvedFromSecondary < 1 {
		t.Errorf("ResolvedFromSecondary = %d, want >= 1", got.ResolvedFromSecondary)
	}
}

// TestFailoverTelemetry_GuardRejected: after installing gen 5, a manual Refresh
// sees an older gen 3 from the same (single) upstream. The reject-older guard
// drops it, and the client must emit guardRejected>=1.
func TestFailoverTelemetry_GuardRejected(t *testing.T) {
	cap := newTelemetryCapture(t)
	upstream, gen := mutableUpstream(t)
	gen.Store(5)

	client, err := quonfig.NewClient(
		quonfig.WithSdkKey("test-backend-key"),
		quonfig.WithAPIURLs([]string{upstream.URL}),
		quonfig.WithTelemetryURL(cap.server.URL),
		quonfig.WithTelemetrySyncInterval(time.Minute),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithInitTimeout(8*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	awaitReady(t, client)
	if got := client.HeldGeneration(); got != 5 {
		t.Fatalf("HeldGeneration = %d, want 5", got)
	}

	// Serve an older generation; the guard must reject it.
	gen.Store(3)
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := client.HeldGeneration(); got != 5 {
		t.Fatalf("HeldGeneration after older refresh = %d, want 5 (guard should reject)", got)
	}
	client.Close() // flushes telemetry

	got := cap.get()
	if got.GuardRejected < 1 {
		t.Errorf("GuardRejected = %d, want >= 1", got.GuardRejected)
	}
	if got.ResolvedFromPrimary < 1 {
		t.Errorf("ResolvedFromPrimary = %d, want >= 1 (init installed from primary)", got.ResolvedFromPrimary)
	}
}
