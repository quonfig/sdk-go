package quonfig

import "testing"

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
