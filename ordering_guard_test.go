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
