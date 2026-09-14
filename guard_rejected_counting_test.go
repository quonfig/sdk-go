package quonfig

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
)

// guardRejected counting rule (qfg-rr5b, decided 2026-09-11, all six backend
// SDKs): only a STRICTLY older payload — incoming Meta.Generation < held —
// counts as guardRejected. An EQUAL-generation re-delivery is a silent no-op:
// still not installed, still advances liveness exactly where it did before,
// but NOT counted.
//
// Why: two server behaviours legitimately re-deliver the envelope the client
// already holds, at the same generation — api-delivery's SSE sendInitialConfig
// re-sends the current envelope on every connect, and a config poll whose
// per-leg ETag slot is empty (fresh transport, reconnect, fallback-poller
// engage fetch) answers a full 200 instead of a 304. Counting those made
// guardRejected — which feeds the sdk_failover alerting signal, where it is
// supposed to mean "a leg tried to move us backwards" — read non-zero for
// every healthy client.
//
// These tests drive both install paths that carry the counter: the HTTP
// fetch path (fetchAndInstall) and the SSE path (handleSSEEnvelope, the
// OnEnvelope sink for every parsed SSE message). They assert on the real
// telemetry wire so a rename of the JSON field would fail here too.

// failoverWire mirrors the api-telemetry failover event field names.
type failoverWire struct {
	HedgeFired            int64 `json:"hedgeFired"`
	GuardRejected         int64 `json:"guardRejected"`
	ResolvedFromPrimary   int64 `json:"resolvedFromPrimary"`
	ResolvedFromSecondary int64 `json:"resolvedFromSecondary"`
	ResolvedFromLkg       int64 `json:"resolvedFromLkg"`
}

// failoverWireCapture accumulates the failover counters across every telemetry
// POST it receives, and counts how many failover events arrived (so a test can
// prove the telemetry path was live rather than silently disabled).
type failoverWireCapture struct {
	server *httptest.Server
	mu     sync.Mutex
	total  failoverWire
	events int
}

func newFailoverWireCapture(t *testing.T) *failoverWireCapture {
	t.Helper()
	c := &failoverWireCapture{}
	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Events []struct {
				Failover *failoverWire `json:"failover"`
			} `json:"events"`
		}
		_ = json.Unmarshal(body, &payload)
		c.mu.Lock()
		for _, ev := range payload.Events {
			if ev.Failover != nil {
				c.events++
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

func (c *failoverWireCapture) get() (failoverWire, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total, c.events
}

// coldETagUpstream serves whatever generation `gen` currently holds with a
// UNIQUE ETag per request, so every fetch is a full 200 and never a 304. This
// is exactly the shape qfg-rr5b describes: a leg whose ETag slot is cold
// re-delivers the envelope the client already holds.
func coldETagUpstream(t *testing.T) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var gen atomic.Int64
	var reqs atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g := gen.Load()
		w.Header().Set("ETag", fmt.Sprintf(`"gen-%d-req-%d"`, g, reqs.Add(1)))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(guardCountingEnvelopeJSON(int(g))))
	}))
	t.Cleanup(srv.Close)
	return srv, &gen
}

func guardCountingEnvelopeJSON(generation int) string {
	return fmt.Sprintf(
		`{"configs":[],"meta":{"version":"gen-%d","environment":"Production","generation":%d}}`,
		generation, generation,
	)
}

func guardCountingEnvelope(generation int) *ConfigEnvelope {
	return &ConfigEnvelope{Meta: Meta{
		Version:     fmt.Sprintf("gen-%d", generation),
		Environment: "Production",
		Generation:  generation,
	}}
}

// guardCountingClient builds a client with telemetry pointed at the capture
// server and every background refresher off ("no-background-refresh" mode), so
// the only fetches are init and the explicit Refresh() a test makes — which
// keeps the counter assertions exact.
func guardCountingClient(t *testing.T, upstreamURL, telemetryURL string) *Client {
	t.Helper()
	client, err := NewClient(
		WithSdkKey("test-backend-key"),
		WithAPIURLs([]string{upstreamURL}),
		WithTelemetryURL(telemetryURL),
		WithTelemetrySyncInterval(time.Minute),
		WithSSE(false),
		WithFallbackPoll(false, 0),
		WithInitTimeout(8*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// Any resolve call awaits initialization internally.
	_, _, _ = client.GetStringValue("any.key", nil)
	return client
}

// TestSameGenerationRedeliveryOnSSEPathIsNotCountedAsGuardRejected: an
// established client receives the envelope it already holds, at the SAME
// generation, on the SSE path (what api-delivery's sendInitialConfig does on
// every reconnect). It must not install, it must still advance liveness, and it
// must NOT be counted as guardRejected (qfg-rr5b).
func TestSameGenerationRedeliveryOnSSEPathIsNotCountedAsGuardRejected(t *testing.T) {
	capture := newFailoverWireCapture(t)
	upstream, gen := coldETagUpstream(t)
	gen.Store(42)

	client := guardCountingClient(t, upstream.URL, capture.server.URL)
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("after init: HeldGeneration = %d, want 42", got)
	}
	installs := client.ConfigInstallCount()
	before := client.LastSuccessfulRefresh()

	// The SSE reconnect resend: same generation, already held.
	time.Sleep(5 * time.Millisecond)
	client.handleSSEEnvelope(guardCountingEnvelope(42))

	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("HeldGeneration = %d, want 42 (same-generation message must not install)", got)
	}
	if got := client.ConfigInstallCount(); got != installs {
		t.Fatalf("install count %d -> %d, want unchanged", installs, got)
	}
	after := client.LastSuccessfulRefresh()
	if !after.After(before) {
		t.Fatalf("LastSuccessfulRefresh = %s, want > %s (a received-and-processed SSE message still proves liveness)", after, before)
	}

	client.Close() // flushes telemetry

	got, events := capture.get()
	if events == 0 {
		t.Fatalf("no failover telemetry event arrived at all — the assertion below would be vacuous")
	}
	if got.GuardRejected != 0 {
		t.Errorf("GuardRejected = %d, want 0: an equal-generation SSE re-delivery must not be counted (qfg-rr5b)", got.GuardRejected)
	}
	if got.ResolvedFromPrimary < 1 {
		t.Errorf("ResolvedFromPrimary = %d, want >= 1 (init installed from the primary)", got.ResolvedFromPrimary)
	}
}

// TestSameGenerationRedeliveryOnHTTPPathIsNotCountedAsGuardRejected: the same
// rule on the HTTP fetch path — a cold-ETag poll answers a full 200 at the
// generation already held. Not installed, liveness still advances, not counted.
func TestSameGenerationRedeliveryOnHTTPPathIsNotCountedAsGuardRejected(t *testing.T) {
	capture := newFailoverWireCapture(t)
	upstream, gen := coldETagUpstream(t)
	gen.Store(42)

	client := guardCountingClient(t, upstream.URL, capture.server.URL)
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("after init: HeldGeneration = %d, want 42", got)
	}
	installs := client.ConfigInstallCount()
	before := client.LastSuccessfulRefresh()

	// Unique ETag per response, so this is a full 200 at the same generation.
	time.Sleep(5 * time.Millisecond)
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("HeldGeneration = %d, want 42 (same-generation 200 must not install)", got)
	}
	if got := client.ConfigInstallCount(); got != installs {
		t.Fatalf("install count %d -> %d, want unchanged", installs, got)
	}
	after := client.LastSuccessfulRefresh()
	if !after.After(before) {
		t.Fatalf("LastSuccessfulRefresh = %s, want > %s (the fetch succeeded; only the install was a no-op)", after, before)
	}

	client.Close()

	got, events := capture.get()
	if events == 0 {
		t.Fatalf("no failover telemetry event arrived at all — the assertion below would be vacuous")
	}
	if got.GuardRejected != 0 {
		t.Errorf("GuardRejected = %d, want 0: an equal-generation 200 must not be counted (qfg-rr5b)", got.GuardRejected)
	}
	if got.ResolvedFromPrimary < 1 {
		t.Errorf("ResolvedFromPrimary = %d, want >= 1 (init installed from the primary)", got.ResolvedFromPrimary)
	}
}

// TestStrictlyOlderRedeliveryOnHTTPPathIsCountedAsGuardRejected pins the
// signal that guardRejected exists for: a leg served a STRICTLY older payload
// and tried to move the client backwards. Exactly one fetch, exactly one count.
func TestStrictlyOlderRedeliveryOnHTTPPathIsCountedAsGuardRejected(t *testing.T) {
	capture := newFailoverWireCapture(t)
	upstream, gen := coldETagUpstream(t)
	gen.Store(42)

	client := guardCountingClient(t, upstream.URL, capture.server.URL)
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("after init: HeldGeneration = %d, want 42", got)
	}
	installs := client.ConfigInstallCount()

	gen.Store(41) // strictly older than held
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("HeldGeneration = %d, want 42 (the guard must still reject an older payload)", got)
	}
	if got := client.ConfigInstallCount(); got != installs {
		t.Fatalf("install count %d -> %d, want unchanged", installs, got)
	}

	client.Close()

	got, _ := capture.get()
	if got.GuardRejected != 1 {
		t.Errorf("GuardRejected = %d, want exactly 1 (one strictly older 200)", got.GuardRejected)
	}
}

// TestStrictlyOlderRedeliveryOnSSEPathIsCountedAsGuardRejected: same, on the
// SSE path.
func TestStrictlyOlderRedeliveryOnSSEPathIsCountedAsGuardRejected(t *testing.T) {
	capture := newFailoverWireCapture(t)
	upstream, gen := coldETagUpstream(t)
	gen.Store(42)

	client := guardCountingClient(t, upstream.URL, capture.server.URL)
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("after init: HeldGeneration = %d, want 42", got)
	}
	installs := client.ConfigInstallCount()

	client.handleSSEEnvelope(guardCountingEnvelope(41))

	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("HeldGeneration = %d, want 42 (the guard must still reject an older message)", got)
	}
	if got := client.ConfigInstallCount(); got != installs {
		t.Fatalf("install count %d -> %d, want unchanged", installs, got)
	}

	client.Close()

	got, _ := capture.get()
	if got.GuardRejected != 1 {
		t.Errorf("GuardRejected = %d, want exactly 1 (one strictly older SSE message)", got.GuardRejected)
	}
}

// TestUnversionedRedeliveryIsNotCountedAsGuardRejected pins the gen<=0
// unversioned carve-out on both paths: a snapshot with generation absent or <= 0
// carries no ordering information, so it still INSTALLS on an established
// client (it can never be "older") and is never counted as guardRejected.
// qfg-rr5b narrowed which rejections are counted; it did not touch the
// carve-out.
func TestUnversionedRedeliveryIsNotCountedAsGuardRejected(t *testing.T) {
	capture := newFailoverWireCapture(t)
	upstream, gen := coldETagUpstream(t)
	gen.Store(42)

	client := guardCountingClient(t, upstream.URL, capture.server.URL)
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("after init: HeldGeneration = %d, want 42", got)
	}
	installs := client.ConfigInstallCount()

	// SSE path: unversioned message installs (carve-out).
	client.handleSSEEnvelope(guardCountingEnvelope(0))
	if got := client.HeldGeneration(); got != 0 {
		t.Fatalf("after unversioned SSE message: HeldGeneration = %d, want 0 (carve-out must install)", got)
	}
	if got := client.ConfigInstallCount(); got != installs+1 {
		t.Fatalf("install count %d -> %d, want %d (carve-out must install)", installs, got, installs+1)
	}

	// HTTP path: unversioned 200 installs too.
	gen.Store(0)
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := client.ConfigInstallCount(); got != installs+2 {
		t.Fatalf("install count = %d, want %d (unversioned 200 must install)", got, installs+2)
	}

	client.Close()

	got, _ := capture.get()
	if got.GuardRejected != 0 {
		t.Errorf("GuardRejected = %d, want 0 (an unversioned snapshot installs; it is never a rejection)", got.GuardRejected)
	}
}
