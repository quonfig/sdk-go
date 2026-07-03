package quonfig

import (
	"testing"
	"time"
)

// WS2.4 (qfg-41nh.11), SSE side: a received-and-processed SSE message counts
// as a successful refresh whether it installs or is guard-no-op'd (the server
// proved live and the held config current). Connection-level SSE failures
// never touch the stamp — they only flow through OnStateChange, which does
// not stamp. This drives handleSSEEnvelope directly (the OnEnvelope sink for
// every parsed SSE message).
func TestHandleSSEEnvelopeStampsGuardNoOp(t *testing.T) {
	client, err := NewClient(
		WithAPIURLs([]string{"https://example.test"}),
		WithAllTelemetryDisabled(),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	env42 := &ConfigEnvelope{Meta: Meta{Version: "gen-42", Environment: "Test", Generation: 42}}

	// First message: fresh client, installs, stamps.
	client.handleSSEEnvelope(env42)
	first := client.LastSuccessfulRefresh()
	if first.IsZero() {
		t.Fatalf("after first SSE message (installed): LastSuccessfulRefresh is zero, want a stamp")
	}
	installs := client.ConfigInstallCount()

	// Same generation again: guard no-op — no install, but the message was
	// received and processed, so liveness must advance.
	time.Sleep(5 * time.Millisecond)
	client.handleSSEEnvelope(env42)
	second := client.LastSuccessfulRefresh()
	if !second.After(first) {
		t.Fatalf("after guard-no-op SSE message: LastSuccessfulRefresh = %s, want > %s", second, first)
	}
	if got := client.ConfigInstallCount(); got != installs {
		t.Fatalf("install count %d -> %d, want unchanged (same-generation SSE message must not re-install)", installs, got)
	}
}
