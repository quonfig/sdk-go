//go:build chaos

package quonfig

import "testing"

// An inject, clear or process event this rig cannot perform must fail the
// scenario (or SKIP it with a reason), never turn into a silent no-op that
// lets the expectations pass against an undisturbed client.
func TestChaosEventValidation(t *testing.T) {
	ms := 1000
	status := 401
	cases := []struct {
		name     string
		ev       chaosEvent
		wantErr  bool
		wantSkip bool
	}{
		{"known inject", chaosEvent{Inject: &chaosInject{Name: "s", SSEDownMs: &ms}}, false, false},
		{"unknown inject shape", chaosEvent{Inject: &chaosInject{Name: "x"}}, true, false},
		{"failover-only inject in main rig", chaosEvent{Inject: &chaosInject{PrimaryRefusedMs: &ms}}, true, false},
		{"sse_http_status needs HTTP proxy", chaosEvent{Inject: &chaosInject{SSEHTTPStatus: &status}}, false, true},
		{"known process", chaosEvent{Process: &chaosProc{Action: "kill_sse_proxy"}}, false, false},
		{"unknown process", chaosEvent{Process: &chaosProc{Action: "restart_api_delivery"}}, true, false},
		{"clear of unknown inject", chaosEvent{Clear: "never_injected"}, true, false},
		{"empty event", chaosEvent{}, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			skip, err := validateChaosEvents([]chaosEvent{tc.ev})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if (skip != "") != tc.wantSkip {
				t.Fatalf("skip = %q, wantSkip %v", skip, tc.wantSkip)
			}
		})
	}
	// A clear naming an earlier inject is fine.
	if _, err := validateChaosEvents([]chaosEvent{
		{Inject: &chaosInject{Name: "stall", SSEDownMs: &ms}},
		{Clear: "stall"},
	}); err != nil {
		t.Fatalf("clear of an earlier inject: %v", err)
	}
	// The failover rig handles only its self-restoring primary/SSE aliases.
	if err := validateFailoverChaosEvents([]chaosEvent{{Inject: &chaosInject{SSELatencyMs: &ms}}}); err == nil {
		t.Fatal("failover rig accepted sse_latency_ms, which it cannot apply")
	}
	if err := validateFailoverChaosEvents([]chaosEvent{{Clear: "x"}}); err == nil {
		t.Fatal("failover rig accepted a clear event, which it ignores")
	}
	if err := validateFailoverChaosEvents([]chaosEvent{{Inject: &chaosInject{PrimaryHangMs: &ms}}}); err != nil {
		t.Fatalf("failover rig rejected primary_hang_ms: %v", err)
	}
}
