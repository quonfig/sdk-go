package quonfig

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// qfg-eefx, SSE path: handleSSEEnvelope holds refreshMu around the install, so
// an OnConfigUpdate that calls Refresh() must not run until it is released.
// Drives handleSSEEnvelope directly (the OnEnvelope sink of the SSE reader).
func TestOnConfigUpdateCanCallRefreshOnSSEInstall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"configs":[],"meta":{"version":"gen-1","environment":"Test","generation":1}}`))
	}))
	t.Cleanup(srv.Close)

	var armed atomic.Bool
	var cbRefreshReturned atomic.Bool
	var clientRef atomic.Pointer[Client]
	client, err := NewClient(
		WithSdkKey("k"),
		WithAPIURLs([]string{srv.URL}),
		WithAllTelemetryDisabled(),
		WithSSE(false),
		WithFallbackPoll(false, 0),
		WithInitTimeout(2*time.Second),
		WithOnConfigUpdate(func() {
			if armed.CompareAndSwap(true, false) {
				_ = clientRef.Load().Refresh()
				cbRefreshReturned.Store(true)
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	clientRef.Store(client)
	t.Cleanup(client.Close)
	_, _, _ = client.GetStringValue("any.key", nil) // await init

	armed.Store(true)
	done := make(chan struct{})
	go func() {
		defer close(done)
		client.handleSSEEnvelope(&ConfigEnvelope{Meta: Meta{Version: "gen-5", Environment: "Test", Generation: 5}})
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("handleSSEEnvelope still blocked after 3s (deadlock: OnConfigUpdate ran under refreshMu)")
	}
	if !cbRefreshReturned.Load() {
		t.Fatalf("OnConfigUpdate's Refresh() did not return before handleSSEEnvelope returned")
	}
	if got := client.HeldGeneration(); got != 5 {
		t.Fatalf("held generation = %d, want 5 (the gen-1 Refresh must be rejected as older)", got)
	}
}
