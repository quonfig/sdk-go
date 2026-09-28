package quonfig

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// qfg-9dxb.3 (audit H2) — a 200 (or SSE event) whose body is not a config
// envelope must never be installed. Before the fix any JSON object decoded to
// an empty envelope with generation 0, the unversioned carve-out installed it
// on an established client, and every key fell back to caller defaults.

const validEnvelopeWithFlag = `{"configs":[{"id":"1","key":"flag.bool","type":"feature_flag","valueType":"bool","default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":true}}]}}],"meta":{"version":"v7","environment":"Production","generation":7}}`

func newValidationClient(t *testing.T, urls []string) *Client {
	t.Helper()
	client, err := NewClient(
		WithSdkKey("test-backend-key"),
		WithAPIURLs(urls),
		WithAllTelemetryDisabled(),
		WithSSE(false),
		WithFallbackPoll(false, 0),
		WithInitTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	_, _, _ = client.GetStringValue("any.key", nil)
	return client
}

func TestNonEnvelope200DoesNotWipeEstablishedClient(t *testing.T) {
	for _, body := range []string{`{}`, `{"error":"upstream maintenance"}`, `{"meta":{}}`, `{"configs":[]}`} {
		t.Run(body, func(t *testing.T) {
			var junk atomic.Bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if junk.Load() {
					w.Header().Set("ETag", `"junk"`)
					_, _ = w.Write([]byte(body))
					return
				}
				w.Header().Set("ETag", `"v7"`)
				_, _ = w.Write([]byte(validEnvelopeWithFlag))
			}))
			t.Cleanup(srv.Close)

			client := newValidationClient(t, []string{srv.URL})
			if v, ok, _ := client.GetBoolValue("flag.bool", nil); !v || !ok {
				t.Fatalf("setup: flag.bool=%v ok=%v, want true/true", v, ok)
			}
			installs := client.ConfigInstallCount()

			junk.Store(true)
			if err := client.Refresh(); err == nil {
				t.Errorf("Refresh returned nil for a non-envelope 200, want a leg error")
			}
			if got := client.ConfigInstallCount(); got != installs {
				t.Errorf("install count %d -> %d, want no install of a non-envelope", installs, got)
			}
			if v, ok, err := client.GetBoolValue("flag.bool", nil); !v || !ok {
				t.Errorf("after non-envelope 200: flag.bool=%v ok=%v err=%v, want the last known value true", v, ok, err)
			}
			if got := client.HeldGeneration(); got != 7 {
				t.Errorf("HeldGeneration = %d, want 7", got)
			}
		})
	}
}

// A non-envelope 200 from the primary is a leg error, so the hedge fires the
// secondary and the client installs the secondary's valid envelope.
func TestNonEnvelope200OnPrimaryFailsOverToSecondary(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"junk"`)
		_, _ = w.Write([]byte(`{"error":"maintenance"}`))
	}))
	t.Cleanup(primary.Close)
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v7"`)
		_, _ = w.Write([]byte(validEnvelopeWithFlag))
	}))
	t.Cleanup(secondary.Close)

	client := newValidationClient(t, []string{primary.URL, secondary.URL})
	if v, ok, err := client.GetBoolValue("flag.bool", nil); !v || !ok {
		t.Fatalf("flag.bool=%v ok=%v err=%v, want true from the secondary", v, ok, err)
	}
	if got := client.ResolvedFrom(); got != "secondary" {
		t.Errorf("ResolvedFrom = %q, want secondary", got)
	}
}

// The ETag of a rejected 200 must not be stored: otherwise the next request
// sends If-None-Match for the junk body, the server 304s, and the junk pins
// itself (audit M3/H2).
func TestNonEnvelope200DoesNotStoreETag(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("If-None-Match"))
		mu.Unlock()
		w.Header().Set("ETag", `"junk"`)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	tr := newRuntimeTransport([]string{srv.URL}, "test-key", nil)
	for i := 0; i < 2; i++ {
		if lr := tr.fetchFromURLAt(context.Background(), 0, 2*time.Second); lr.Err == nil {
			t.Errorf("fetch %d: want an error for a non-envelope 200, got %+v", i, lr.Res)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	for i, h := range seen {
		if h != "" {
			t.Errorf("request %d sent If-None-Match %q, want none (junk ETag must not be stored)", i, h)
		}
	}
}

// A payload with version+environment but no generation installs every time on
// a client that has never received a real generation (held generation 0); a
// client already holding a real generation does not (qfg-9dxb.9).
func TestQfgServeStylePayloadStillInstalls(t *testing.T) {
	var serve atomic.Bool
	var n atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serve.Load() {
			w.Header().Set("ETag", fmt.Sprintf(`"serve-%d"`, n.Add(1)))
			_, _ = w.Write([]byte(`{"configs":[],"meta":{"version":"local-abc123","environment":"development"}}`))
			return
		}
		w.Header().Set("ETag", `"v7"`)
		_, _ = w.Write([]byte(validEnvelopeWithFlag))
	}))
	t.Cleanup(srv.Close)

	client := newValidationClient(t, []string{srv.URL})
	installs := client.ConfigInstallCount()
	serve.Store(true)
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := client.ConfigInstallCount(); got != installs {
		t.Fatalf("install count %d -> %d, want a qfg-serve payload NOT to install over held generation 7", installs, got)
	}
	if got := client.HeldGeneration(); got != 7 {
		t.Errorf("HeldGeneration = %d, want 7", got)
	}

	// A fresh client seeds off a qfg-serve payload and keeps installing them.
	fresh := newValidationClient(t, []string{srv.URL})
	if got := fresh.ConfigInstallCount(); got != 1 {
		t.Errorf("fresh client install count = %d, want 1", got)
	}
	if err := fresh.Refresh(); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := fresh.ConfigInstallCount(); got != 2 {
		t.Errorf("fresh client install count after refresh = %d, want 2 (gen-0-only client keeps installing)", got)
	}
}

// SSE: a non-envelope event is dropped like malformed JSON; a valid one after
// it is still delivered.
func TestSSEDropsNonEnvelopeEvents(t *testing.T) {
	var got []*ConfigEnvelope
	c := newSSEClient(sseClientConfig{
		OnEnvelope: func(env *ConfigEnvelope) { got = append(got, env) },
	})
	stream := strings.Join([]string{
		`data: {}`, ``,
		`data: {"error":"maintenance"}`, ``,
		`data: {"configs":[],"meta":{"environment":"Production"}}`, ``,
		`data: {"configs":[],"meta":{"version":"v9","environment":"Production","generation":9}}`, ``,
	}, "\n") + "\n"
	_ = c.parseStream(strings.NewReader(stream))
	if len(got) != 1 {
		t.Fatalf("OnEnvelope called %d times, want 1 (only the valid envelope)", len(got))
	}
	if got[0].Meta.Version != "v9" {
		t.Errorf("delivered version %q, want v9", got[0].Meta.Version)
	}
}

// qfg-9dxb.9 follow-up: a gen<=0 200 dropped by the guard must not leave its
// ETag remembered. api-delivery can later repair the generation for the SAME
// sha (same ETag); if the dropped response's ETag were kept, every later poll
// would 304 and the client would stay on the old config until the next commit.
func TestDroppedGenZero200DoesNotPinItsETag(t *testing.T) {
	envelope := func(flag bool, gen int) string {
		return fmt.Sprintf(`{"configs":[{"id":"1","key":"flag.bool","type":"feature_flag","valueType":"bool","default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":%t}}]}}],"meta":{"version":"v","environment":"Production","generation":%d}}`, flag, gen)
	}
	var phase atomic.Int32 // 0: A@gen5, 1: B@gen0, 2: B@gen6 (same ETag as phase 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch phase.Load() {
		case 0:
			w.Header().Set("ETag", `"shaA"`)
			_, _ = w.Write([]byte(envelope(true, 5)))
		case 1:
			w.Header().Set("ETag", `"shaB"`)
			_, _ = w.Write([]byte(envelope(false, 0)))
		default:
			if r.Header.Get("If-None-Match") == `"shaB"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"shaB"`)
			_, _ = w.Write([]byte(envelope(false, 6)))
		}
	}))
	t.Cleanup(srv.Close)

	client := newValidationClient(t, []string{srv.URL})
	if v, ok, _ := client.GetBoolValue("flag.bool", nil); !v || !ok || client.HeldGeneration() != 5 {
		t.Fatalf("setup: flag.bool=%v ok=%v held=%d, want true/true/5", v, ok, client.HeldGeneration())
	}

	phase.Store(1)
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh (gen 0): %v", err)
	}
	if v, _, _ := client.GetBoolValue("flag.bool", nil); !v || client.HeldGeneration() != 5 {
		t.Fatalf("gen-0 200 must be ignored: flag.bool=%v held=%d, want true/5", v, client.HeldGeneration())
	}

	phase.Store(2)
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh (gen 6): %v", err)
	}
	if v, _, _ := client.GetBoolValue("flag.bool", nil); v || client.HeldGeneration() != 6 {
		t.Fatalf("after repaired generation: flag.bool=%v held=%d, want false/6 (dropped gen-0 ETag must not pin a 304)", v, client.HeldGeneration())
	}
}
