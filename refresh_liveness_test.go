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

// WS2.4 (qfg-41nh.11): LastSuccessfulRefresh is a LIVENESS signal, not an
// install counter. A fetch that completes successfully at the HTTP layer —
// 200 installed, 200 rejected by the ordering guard as equal-or-older, or
// 304 Not Modified — proves the config source is reachable and the held
// config is current, so it must advance the stamp. Transport errors must
// not. Without this, a healthy long-lived client parked on 304s under-reports
// liveness: LastSuccessfulRefresh freezes even though every fetch succeeds
// (the root mechanism behind the qfg-sc90 chaos red).

// TestLastSuccessfulRefreshStampsInitAnd304 pins two stamps:
//   - the init install stamps (it happens before the supervisor exists, which
//     historically swallowed it), and
//   - a later refresh answered 304 Not Modified stamps, because the fetch
//     succeeded and the held config was confirmed current.
func TestLastSuccessfulRefreshStampsInitAnd304(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"gen-42"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"gen-42"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(orderingEnvelopeJSON(42)))
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
	first := client.LastSuccessfulRefresh()
	if first.IsZero() {
		t.Fatalf("after init install: LastSuccessfulRefresh is zero, want a stamp (init fetch succeeded and installed)")
	}

	time.Sleep(5 * time.Millisecond)
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	second := client.LastSuccessfulRefresh()
	if !second.After(first) {
		t.Fatalf("after 304 refresh: LastSuccessfulRefresh = %s, want > %s (a 304 IS a successful refresh)", second, first)
	}
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("held generation = %d, want 42 (304 must not touch the installed config)", got)
	}
}

// TestLastSuccessfulRefreshStampsGuardRejected200 pins the guard-rejected
// case: an established client fails over to a leg serving an OLDER
// generation. The ordering guard correctly drops the payload — but the fetch
// itself succeeded, so liveness must still advance.
func TestLastSuccessfulRefreshStampsGuardRejected200(t *testing.T) {
	var primaryDead atomic.Bool
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if primaryDead.Load() {
			http.Error(w, "primary refused", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("ETag", `"gen-42"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(orderingEnvelopeJSON(42)))
	}))
	defer primary.Close()

	// The secondary varies its ETag per request so every fetch is a full 200
	// (never a 304) — this test isolates the guard-rejected path.
	var secondaryReqs atomic.Int64
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", fmt.Sprintf(`"gen-41-%d"`, secondaryReqs.Add(1)))
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
		t.Fatalf("after init: held generation = %d, want 42", got)
	}
	installs := client.ConfigInstallCount()
	first := client.LastSuccessfulRefresh()

	primaryDead.Store(true)
	time.Sleep(5 * time.Millisecond)
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	second := client.LastSuccessfulRefresh()
	if !second.After(first) {
		t.Fatalf("after guard-rejected 200: LastSuccessfulRefresh = %s, want > %s (the fetch succeeded; only the install was a no-op)", second, first)
	}
	if got := client.HeldGeneration(); got != 42 {
		t.Fatalf("held generation = %d, want 42 (guard must still reject the older payload)", got)
	}
	if got := client.ConfigInstallCount(); got != installs {
		t.Fatalf("install count %d -> %d, want unchanged (stamp must not come from an install)", installs, got)
	}
}

// TestLastSuccessfulRefreshNotStampedOnError pins the negative: a refresh
// whose every leg fails must NOT advance the stamp — the whole point of the
// signal is distinguishing "successfully confirmed current" from "cannot
// reach the config source".
func TestLastSuccessfulRefreshNotStampedOnError(t *testing.T) {
	var dead atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if dead.Load() {
			http.Error(w, "gone", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("ETag", `"gen-42"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(orderingEnvelopeJSON(42)))
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
	first := client.LastSuccessfulRefresh()
	if first.IsZero() {
		t.Fatalf("after init install: LastSuccessfulRefresh is zero, want a stamp")
	}

	dead.Store(true)
	time.Sleep(5 * time.Millisecond)
	if err := client.Refresh(); err == nil {
		t.Fatalf("Refresh: expected error with every leg failing, got nil")
	}
	if got := client.LastSuccessfulRefresh(); !got.Equal(first) {
		t.Fatalf("after failed refresh: LastSuccessfulRefresh = %s, want unchanged %s (errors must not stamp)", got, first)
	}
}
