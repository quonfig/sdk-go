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

// Parallel-failover hedge unit tests (qfg-7h5d.1.14). These pin the behaviors
// the chaos ordering scenarios assert (o01 cold-standby, o03 heal-forward, o05
// secondary-newer-wins) at the unit level, where a per-leg request counter can
// prove the "secondary is never contacted on a fast primary" contract that the
// chaos rig (no server-side counter) cannot. They use only the public API and
// default hedge timings, so the file also compiles + runs against the
// pre-hedge sequential transport to capture the RED baseline.

func hedgeEnvelopeJSON(generation int) string {
	return fmt.Sprintf(
		`{"configs":[],"meta":{"version":"gen-%d","environment":"Production","generation":%d}}`,
		generation, generation,
	)
}

// hedgeUpstream is an httptest server pinned to a generation, optionally delayed
// by `delay` before it answers, counting every request it receives.
func hedgeUpstream(t *testing.T, generation int, delay time.Duration) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if delay > 0 {
			time.Sleep(delay)
		}
		w.Header().Set("ETag", fmt.Sprintf(`"gen-%d"`, generation))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(hedgeEnvelopeJSON(generation)))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func newHedgeClient(t *testing.T, primaryURL, secondaryURL string) *quonfig.Client {
	t.Helper()
	client, err := quonfig.NewClient(
		quonfig.WithSdkKey("test-backend-key"),
		quonfig.WithAPIURLs([]string{primaryURL, secondaryURL}),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithInitTimeout(8*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

// awaitReady blocks until the first install latches readiness.
func awaitReady(t *testing.T, client *quonfig.Client) {
	t.Helper()
	_, _, _ = client.GetStringValue("any.key", nil)
}

// pollUntilGeneration waits up to `within` for the held generation to reach
// `want`, polling so a background heal-forward install is observed.
func pollUntilGeneration(t *testing.T, client *quonfig.Client, want int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if client.HeldGeneration() == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("held generation did not reach %d within %s (last = %d)", want, within, client.HeldGeneration())
}

// TestHedgeFastPrimaryNeverContactsSecondary is the unit-level o01: both legs
// healthy and fast, secondary newer. A fast primary answers well inside the
// hedge delay, so the secondary is NEVER contacted (cold standby, zero extra
// load). The client holds the primary's (lower) generation and resolvedFrom
// stays 'primary'. This is the cold-standby proof the chaos rig cannot make.
func TestHedgeFastPrimaryNeverContactsSecondary(t *testing.T) {
	primary, primaryHits := hedgeUpstream(t, 41, 0)
	secondary, secondaryHits := hedgeUpstream(t, 42, 0)

	client := newHedgeClient(t, primary.URL, secondary.URL)
	awaitReady(t, client)

	if got := client.HeldGeneration(); got != 41 {
		t.Fatalf("held generation = %d, want 41 (fast primary wins; secondary's 42 must not be installed)", got)
	}
	if got := client.ResolvedFrom(); got != "primary" {
		t.Fatalf("resolvedFrom = %q, want \"primary\"", got)
	}
	if got := secondaryHits.Load(); got != 0 {
		t.Fatalf("secondary contacted %d times, want 0 (cold standby — a fast primary must never trigger the hedge)", got)
	}
	if got := primaryHits.Load(); got == 0 {
		t.Fatal("primary was never contacted")
	}
}

// TestHedgeSecondaryNewerWins is the unit-level o05 and the cleanest RED→GREEN
// discriminator: the primary is SLOW and serves the OLDER generation (41); the
// secondary is fast and serves the NEWER generation (42). The hedge fires the
// secondary once the hedge delay elapses (primary still slow), installs 42, and
// when the slow primary's older 41 lands late the reject-older guard drops it.
//
// On the pre-hedge sequential transport the primary is tried first; it answers
// (slowly, but inside the per-URL timeout) with 41, the secondary is never
// contacted, and the client holds 41 — so this test is RED. The hedge makes it
// hold 42 (GREEN).
func TestHedgeSecondaryNewerWins(t *testing.T) {
	primary, _ := hedgeUpstream(t, 41, 2500*time.Millisecond)
	secondary, secondaryHits := hedgeUpstream(t, 42, 0)

	client := newHedgeClient(t, primary.URL, secondary.URL)
	awaitReady(t, client)

	// The hedge must have fired the secondary (slow primary) and installed its 42.
	pollUntilGeneration(t, client, 42, 5*time.Second)
	if got := secondaryHits.Load(); got == 0 {
		t.Fatal("secondary was never contacted — the hedge did not fire against the slow primary")
	}

	// The slow primary's older 41 lands late and on every subsequent refresh; the
	// reject-older guard must keep the client on 42.
	for i := 0; i < 3; i++ {
		_ = client.Refresh()
	}
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("held generation = %d after late older primary, want 42 (reject-older must drop the slow 41)", got)
	}
}

// TestHedgeHealsForwardToSlowNewerPrimary is the unit-level o03: the primary is
// SLOW and serves the NEWER generation (42); the secondary is fast and serves
// the OLDER generation (41). The hedge seeds readiness off the secondary's 41,
// then heals forward to the primary's 42 when it lands — reject-older only
// blocks going backward, never forward.
//
// On the pre-hedge sequential transport the secondary is never contacted (the
// slow primary answers first with 42), so secondaryHits == 0 — RED. The hedge
// contacts the secondary in parallel (GREEN).
func TestHedgeHealsForwardToSlowNewerPrimary(t *testing.T) {
	primary, _ := hedgeUpstream(t, 42, 2500*time.Millisecond)
	secondary, secondaryHits := hedgeUpstream(t, 41, 0)

	client := newHedgeClient(t, primary.URL, secondary.URL)
	awaitReady(t, client)

	if got := secondaryHits.Load(); got == 0 {
		t.Fatal("secondary was never contacted — the hedge did not fire against the slow primary")
	}
	// Heal forward to the slow primary's newer 42.
	pollUntilGeneration(t, client, 42, 5*time.Second)
}
