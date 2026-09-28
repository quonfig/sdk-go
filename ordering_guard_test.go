package quonfig_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	quonfig "github.com/quonfig/sdk-go"
)

// orderingEnvelopeJSON returns a minimal config envelope pinned to the given
// Meta.generation watermark (qfg-7h5d.1.1).
func orderingEnvelopeJSON(generation int) string {
	return fmt.Sprintf(
		`{"configs":[],"meta":{"version":"gen-%d","environment":"Production","generation":%d}}`,
		generation, generation,
	)
}

// awaitClientReady blocks until the client has finished its first install (any
// resolve call awaits initialization internally) so HeldGeneration reflects the
// seeded snapshot.
func awaitClientReady(t *testing.T, client *quonfig.Client) {
	t.Helper()
	_, _, _ = client.GetStringValue("any.key", nil)
}

// TestRejectOlderInstallGuard pins the canonical reject-older rule on the
// failover fetch path — the unit-level analogue of chaos scenario o02. A client
// establishes on the primary's newer generation (42); the primary then goes
// dark and refreshes fail over to the secondary, which serves the OLDER
// generation (41). The install guard must drop that payload: install only if
// incoming.Meta.Generation > held. Without the guard installEnvelope installs
// 41 unconditionally and the established client regresses.
func TestRejectOlderInstallGuard(t *testing.T) {
	var primaryDead atomic.Bool

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if primaryDead.Load() {
			http.Error(w, "primary refused", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("ETag", `"primary-42"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(orderingEnvelopeJSON(42)))
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"secondary-41"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(orderingEnvelopeJSON(41)))
	}))
	defer secondary.Close()

	client, err := quonfig.NewClient(
		quonfig.WithSdkKey("test-backend-key"),
		quonfig.WithAPIURLs([]string{primary.URL, secondary.URL}),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	awaitClientReady(t, client)
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("after init: held generation = %d, want 42 (must establish on primary)", got)
	}

	// Primary goes dark; every refresh now fails over to the secondary's OLDER
	// gen 41. The reject-older guard must keep the established client on 42.
	primaryDead.Store(true)
	for i := 0; i < 5; i++ {
		_ = client.Refresh()
	}

	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("after failover to older secondary: held generation = %d, want 42 (reject-older guard must drop gen 41)", got)
	}
}

// contentEnvelopeJSON returns an envelope at the given generation holding one
// bool flag "flag.content" whose value marks which content (NEW=true,
// OLD=false) is installed.
func contentEnvelopeJSON(generation int, value bool) string {
	return fmt.Sprintf(
		`{"configs":[{"id":"1","key":"flag.content","type":"feature_flag","valueType":"bool","default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":%t}}]}}],"meta":{"version":"gen-%d-%t","environment":"Production","generation":%d}}`,
		value, generation, value, generation,
	)
}

// swappableServer serves whatever body is currently stored, with a unique
// ETag per request so the client never gets a 304.
func swappableServer(t *testing.T, body *atomic.Value) *httptest.Server {
	t.Helper()
	var reqs atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", fmt.Sprintf(`"req-%d"`, reqs.Add(1)))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body.Load().(string)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func contentFlag(t *testing.T, client *quonfig.Client) bool {
	t.Helper()
	v, ok, err := client.GetBoolValue("flag.content", nil)
	if err != nil || !ok {
		t.Fatalf("GetBoolValue(flag.content) = %v, ok=%v, err=%v", v, ok, err)
	}
	return v
}

// TestInstallGuardUnversionedDoesNotOverrideHeldGeneration pins qfg-9dxb.9: a
// client holding a real generation must NOT install an unversioned (gen <= 0)
// payload. Pre-watermark servers that sent gen 0 on every payload are long
// gone; gen 0 now only comes from an api-delivery machine whose git object
// store is damaged (rev-count failed), and its content may be OLD. Installing
// it moved the client backward, and because the held generation is not lowered
// (qfg-9dxb.3) the healthy gen-N re-delivery was then rejected as a same-
// generation no-op — stuck on OLD content until gen N+1.
func TestInstallGuardUnversionedDoesNotOverrideHeldGeneration(t *testing.T) {
	var body atomic.Value
	body.Store(contentEnvelopeJSON(42, true)) // gen 42, NEW content

	server := swappableServer(t, &body)
	client, err := quonfig.NewClient(
		quonfig.WithSdkKey("test-backend-key"),
		quonfig.WithAPIURLs([]string{server.URL}),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	awaitClientReady(t, client)
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("after init: held generation = %d, want 42", got)
	}
	if !contentFlag(t, client) {
		t.Fatalf("after init: flag.content = false, want true (NEW content)")
	}

	// A damaged-store machine answers gen 0 with OLD content. It must not install.
	body.Store(contentEnvelopeJSON(0, false))
	installs := client.ConfigInstallCount()
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := client.ConfigInstallCount(); got != installs {
		t.Fatalf("install count %d -> %d after gen-0 payload, want no install (gen<=0 must not override a held real generation)", installs, got)
	}
	if !contentFlag(t, client) {
		t.Fatalf("after gen-0 OLD payload: flag.content = false, want true (client must stay on NEW content)")
	}
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("held generation = %d after gen-0 payload, want 42", got)
	}

	// The healthy gen-42 re-delivery: still NEW (the stuck scenario).
	body.Store(contentEnvelopeJSON(42, true))
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !contentFlag(t, client) {
		t.Fatalf("after gen-42 re-delivery: flag.content = false, want true (client stuck on OLD content)")
	}
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("held generation = %d after gen-42 re-delivery, want 42", got)
	}
}

// TestInstallGuardUnversionedOnlyClientKeepsInstalling pins the other half of
// qfg-9dxb.9: a client that has never received a real generation
// (heldGeneration == 0) keeps installing every unversioned payload, so it
// never freezes on its first snapshot.
func TestInstallGuardUnversionedOnlyClientKeepsInstalling(t *testing.T) {
	var body atomic.Value
	body.Store(contentEnvelopeJSON(0, true))

	server := swappableServer(t, &body)
	client, err := quonfig.NewClient(
		quonfig.WithSdkKey("test-backend-key"),
		quonfig.WithAPIURLs([]string{server.URL}),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	awaitClientReady(t, client)
	if !contentFlag(t, client) {
		t.Fatalf("after init: flag.content = false, want true")
	}

	for i, want := range []bool{false, true, false} {
		body.Store(contentEnvelopeJSON(0, want))
		installs := client.ConfigInstallCount()
		if err := client.Refresh(); err != nil {
			t.Fatalf("Refresh %d: %v", i, err)
		}
		if got := client.ConfigInstallCount(); got != installs+1 {
			t.Fatalf("refresh %d: install count %d -> %d, want %d (a gen-0-only client must install every gen-0 payload)", i, installs, got, installs+1)
		}
		if got := contentFlag(t, client); got != want {
			t.Fatalf("refresh %d: flag.content = %v, want %v", i, got, want)
		}
		if got := client.HeldGeneration(); got != 0 {
			t.Fatalf("refresh %d: held generation = %d, want 0", i, got)
		}
	}
}

// TestInstallGuardHealsForwardAndSeeds covers the other three ordering
// invariants the guard must preserve:
//   - a fresh client seeds off whatever arrives first, even an older generation
//   - a later, newer generation heals forward (reject-older only blocks going
//     backward) (o03)
//   - a same-generation second snapshot is a no-op — no second install (o04)
func TestInstallGuardHealsForwardAndSeeds(t *testing.T) {
	var gen atomic.Int64
	gen.Store(41) // fresh client seeds off the older snapshot first

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g := int(gen.Load())
		// Distinct ETag per generation so a bumped generation isn't masked as a
		// 304 by the transport's shared If-None-Match.
		w.Header().Set("ETag", fmt.Sprintf(`"gen-%d"`, g))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(orderingEnvelopeJSON(g)))
	}))
	defer server.Close()

	client, err := quonfig.NewClient(
		quonfig.WithSdkKey("test-backend-key"),
		quonfig.WithAPIURLs([]string{server.URL}),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer client.Close()

	awaitClientReady(t, client)
	if got := client.HeldGeneration(); got != 41 {
		t.Fatalf("fresh client should seed off gen 41, got %d", got)
	}
	seedInstalls := client.ConfigInstallCount()

	// Same generation served again: no-op, no second install (o04).
	for i := 0; i < 3; i++ {
		_ = client.Refresh()
	}
	if got := client.ConfigInstallCount(); got != seedInstalls {
		t.Fatalf("same-generation refresh flapped: install count %d -> %d (want no change)", seedInstalls, got)
	}
	if got := client.HeldGeneration(); got != 41 {
		t.Fatalf("same-generation refresh changed held generation to %d, want 41", got)
	}

	// A newer generation lands: heal forward to 42 (o03).
	gen.Store(42)
	_ = client.Refresh()
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("newer generation should heal forward, held = %d, want 42", got)
	}
}
