package quonfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestFetchConfigsPerURLTimeoutFailsOver is the hang-failover proof for the
// per-URL config-fetch timeout (bead qfg-7h5d.1.4, mirrors chaos scenario
// f02-primary-hang). The primary leg accepts the connection but never responds;
// with a per-URL deadline the primary attempt aborts fast and FetchConfigs
// resolves off the secondary, well inside the much larger parent budget.
//
// Revert check: delete the context.WithTimeout wrapping in fetchFromURL and the
// hung primary starves the secondary until the 4s parent deadline — FetchConfigs
// returns a DeadlineExceeded error and the SourceIndex==1 assertion is never
// reached. So the test fails iff the per-URL mechanism is absent.
func TestFetchConfigsPerURLTimeoutFailsOver(t *testing.T) {
	release := make(chan struct{})

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept the connection, never send a response: hang until either the
		// per-URL deadline cancels the request or the test tears down.
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(primary.Close)
	t.Cleanup(func() { close(release) })

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(secondary.Close)

	tr := newRuntimeTransport([]string{primary.URL, secondary.URL}, "test-key", nil)
	tr.fetchTimeout = 300 * time.Millisecond

	// Parent budget dwarfs the per-URL timeout; only the per-URL deadline can
	// bound the hung primary attempt.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	start := time.Now()
	result, err := tr.FetchConfigs(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("FetchConfigs returned error: %v (elapsed %s) — hung primary not bounded by per-URL timeout", err, elapsed)
	}
	if result == nil || result.Envelope == nil {
		t.Fatalf("expected an envelope resolved from the secondary, got %+v", result)
	}
	if result.SourceIndex != 1 {
		t.Fatalf("expected SourceIndex=1 (secondary leg), got %d", result.SourceIndex)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("FetchConfigs took %s — per-URL timeout did not abort the hung primary fast", elapsed)
	}
}

func TestRuntimeTransportStreamURLDerivation(t *testing.T) {
	tr := newRuntimeTransport(
		[]string{"https://primary.quonfig.com", "http://localhost:8080/"},
		"test-key",
		nil,
	)

	if got, want := len(tr.streamURLs), 2; got != want {
		t.Fatalf("expected %d stream URLs, got %d", want, got)
	}

	// streamURLFor appends the SSE path.
	if got, want := tr.streamURLFor(0), "https://stream.primary.quonfig.com/api/v2/sse/config"; got != want {
		t.Fatalf("streamURLFor(0) = %q, want %q", got, want)
	}
	if got, want := tr.streamURLFor(1), "http://stream.localhost:8080/api/v2/sse/config"; got != want {
		t.Fatalf("streamURLFor(1) = %q, want %q", got, want)
	}
	// Out-of-range returns "".
	if got := tr.streamURLFor(2); got != "" {
		t.Fatalf("streamURLFor(2) out-of-range = %q, want empty", got)
	}

	// Base URLs keep the HTTP pollers pointed at the apiUrl host (unchanged).
	if got, want := tr.baseURLs[0], "https://primary.quonfig.com"; got != want {
		t.Fatalf("baseURLs[0] = %q, want %q", got, want)
	}
	if got, want := tr.baseURLs[1], "http://localhost:8080"; got != want {
		t.Fatalf("baseURLs[1] = %q, want %q", got, want)
	}
}

func TestRuntimeTransportStreamURLOverride(t *testing.T) {
	// When a test override is provided, streamURLs[i] == override for every i.
	tr := newRuntimeTransportWithStreamOverride(
		[]string{"https://primary.quonfig.com", "https://secondary.quonfig.com"},
		"test-key",
		nil,
		"http://127.0.0.1:54321",
	)

	for i := range tr.streamURLs {
		if got, want := tr.streamURLFor(i), "http://127.0.0.1:54321/api/v2/sse/config"; got != want {
			t.Fatalf("streamURLFor(%d) under override = %q, want %q", i, got, want)
		}
	}
	// Base (HTTP polling) URLs are unaffected by the stream override.
	if got, want := tr.baseURLs[0], "https://primary.quonfig.com"; got != want {
		t.Fatalf("baseURLs[0] = %q, want %q", got, want)
	}
}

func TestDefaultOptionsAPIURLs(t *testing.T) {
	// Defaults derive from QUONFIG_DOMAIN (default "quonfig.com") and
	// include both primary and secondary hosts. See options.go
	// apiURLsForDomain. Explicit WithAPIURLs is the escape hatch for
	// callers that want a single host.
	o := defaultOptions()
	if got, want := len(o.APIURLs), 2; got != want {
		t.Fatalf("defaultOptions().APIURLs len = %d, want %d (%v)", got, want, o.APIURLs)
	}
	if got, want := o.APIURLs[0], "https://primary.quonfig.com"; got != want {
		t.Fatalf("defaultOptions().APIURLs[0] = %q, want %q", got, want)
	}
	if got, want := o.APIURLs[1], "https://secondary.quonfig.com"; got != want {
		t.Fatalf("defaultOptions().APIURLs[1] = %q, want %q", got, want)
	}
}
