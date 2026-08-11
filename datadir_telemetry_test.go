package quonfig

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Telemetry must be gated on SDK-key presence, not on mode (qfg-j001).
//
// Port of sdk-node's test/datadir-telemetry.test.ts. Two halves:
//
//  1. Keyless datadir (open-source / no-account path): there is no workspace to
//     attribute telemetry to, so the submitter must never start. Before the fix
//     sdk-go started it anyway and every Close() POSTed to the telemetry
//     endpoint with `Authorization: Basic base64("1:")` — an unauthenticated
//     request, retried up to maxRetries with backoff.
//  2. Datadir WITH a key (the dogfood path — app-quonfig, api-telemetry): must
//     still emit. Don't over-rotate into sdk-ruby's inverse bug, where datadir
//     mode drops telemetry even with a valid key.

// telemetryPostRecorder counts every POST that reaches the telemetry endpoint
// and keeps the raw bodies so a test can assert on what was sent.
type telemetryPostRecorder struct {
	server *httptest.Server
	mu     sync.Mutex
	posts  []string
	auths  []string
}

func newTelemetryPostRecorder(t *testing.T) *telemetryPostRecorder {
	t.Helper()
	r := &telemetryPostRecorder{}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.mu.Lock()
		r.posts = append(r.posts, string(body))
		r.auths = append(r.auths, req.Header.Get("Authorization"))
		r.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(r.server.Close)
	return r
}

func (r *telemetryPostRecorder) snapshot() ([]string, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.posts...), append([]string(nil), r.auths...)
}

// settleTelemetryQueue gives the submitter's consumeQueue goroutine time to
// move recorded evaluations out of the channel and into the aggregator.
// RecordEvaluation/RecordContext are asynchronous (unlike the failover
// counters, which are recorded inline), so a Close() issued immediately after
// an evaluation can flush before the queue item has been aggregated. Without
// this wait the no-key assertion would pass vacuously — the "red" run below
// shows a real POST reaching the server.
func settleTelemetryQueue() {
	time.Sleep(250 * time.Millisecond)
}

func telemetryWorkspaceFixture(t *testing.T) string {
	t.Helper()
	return writeWorkspace(t,
		`{"prod":"Production"}`,
		"configs/welcome-message.json",
		`{
			"id":"cfg-1",
			"key":"welcome-message",
			"type":"config",
			"valueType":"string",
			"sendToClientSdk":false,
			"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"string","value":"hello"}}]},
			"environments":[
				{"id":"Production","rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"string","value":"hola"}}]}
			]
		}`,
	)
}

// The no-key half: datadir mode with no SDK key must not submit anything.
func TestDatadirTelemetry_NoSdkKeyEmitsNothing(t *testing.T) {
	t.Setenv("QUONFIG_BACKEND_SDK_KEY", "")
	rec := newTelemetryPostRecorder(t)

	client, err := NewClient(
		// No WithSdkKey — the datadir is the only credential-equivalent.
		WithDataDir(telemetryWorkspaceFixture(t)),
		WithEnvironment("Production"),
		WithTelemetryURL(rec.server.URL),
		WithTelemetrySyncInterval(time.Minute),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	value, ok, err := client.GetStringValue("welcome-message", nil)
	if err != nil || !ok || value != "hola" {
		t.Fatalf("GetStringValue = (%q, %v, %v), want (hola, true, nil)", value, ok, err)
	}

	settleTelemetryQueue()
	client.Close() // flushes telemetry, if any is pending

	posts, auths := rec.snapshot()
	if len(posts) != 0 {
		t.Fatalf("keyless datadir client sent %d telemetry POST(s) (auth headers %v); want 0. Bodies: %v", len(posts), auths, posts)
	}
	if client.telemetry != nil {
		t.Fatalf("telemetry submitter was started without an SDK key")
	}
}

// The with-key half (the dogfood path): datadir mode plus a real key must
// still submit eval summaries. Guards against the sdk-ruby inverse bug.
func TestDatadirTelemetry_WithSdkKeyStillEmits(t *testing.T) {
	t.Setenv("QUONFIG_BACKEND_SDK_KEY", "")
	rec := newTelemetryPostRecorder(t)

	client, err := NewClient(
		WithSdkKey("test-backend-key"),
		WithDataDir(telemetryWorkspaceFixture(t)),
		WithEnvironment("Production"),
		WithTelemetryURL(rec.server.URL),
		WithTelemetrySyncInterval(time.Minute),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	value, ok, err := client.GetStringValue("welcome-message", nil)
	if err != nil || !ok || value != "hola" {
		t.Fatalf("GetStringValue = (%q, %v, %v), want (hola, true, nil)", value, ok, err)
	}

	settleTelemetryQueue()
	client.Close() // flushes telemetry

	posts, _ := rec.snapshot()
	if len(posts) == 0 {
		t.Fatalf("datadir client WITH an SDK key sent no telemetry; want at least one POST")
	}
	found := false
	for _, body := range posts {
		if strings.Contains(body, "welcome-message") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no telemetry POST carried the evaluated key; bodies: %v", posts)
	}
}

// Unit-level guard on the predicate itself: a non-empty key is required
// regardless of which collectors are on.
func TestTelemetryEnabledRequiresSdkKey(t *testing.T) {
	base := func() Options {
		o := defaultOptions()
		o.TelemetryURL = "https://telemetry.example.com"
		return o
	}

	o := base()
	if o.TelemetryEnabled() {
		t.Error("TelemetryEnabled() = true with an empty APIKey, want false")
	}

	o = base()
	o.APIKey = "sdk-key"
	if !o.TelemetryEnabled() {
		t.Error("TelemetryEnabled() = false with a key and default collectors, want true")
	}

	o = base()
	o.APIKey = "sdk-key"
	o.CollectEvaluationSummaries = false
	o.ContextTelemetryMode = ContextTelemetryNone
	if o.TelemetryEnabled() {
		t.Error("TelemetryEnabled() = true with all collectors off, want false")
	}

	o = base()
	o.APIKey = "sdk-key"
	o.TelemetryURL = ""
	if o.TelemetryEnabled() {
		t.Error("TelemetryEnabled() = true with no telemetry URL, want false")
	}
}
