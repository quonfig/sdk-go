//go:build chaos

package quonfig

import (
	"strings"
	"testing"
)

// Expression-evaluator tests for the chaos harness (qfg-goi1.1.1). They need
// no toxiproxy or api-delivery, and their names match `-run TestChaos`, so
// scripts/run-chaos.sh runs them in the chaos job too.

const lagExpr = "server_metric('quonfig_subscriber_lag_seconds') == 0"

// server_metric used to be stubbed to 0, so `== 0` passed without checking
// anything. It must now be SKIPPED, with the reason.
func TestChaosExprServerMetricIsSkippedNotSilentZero(t *testing.T) {
	ec := &evalCtx{probe: newChaosProbe()}
	for _, expr := range []string{lagExpr, "server_metric('quonfig_subscriber_lag_seconds') > 60"} {
		r := evaluate(expr, ec)
		if r.verdict != verdictSkipped {
			t.Fatalf("%s: verdict=%s why=%q, want skipped", expr, r.verdict, r.why)
		}
		if !strings.Contains(r.why, "SKIPPED") || !strings.Contains(r.why, "OTLP") || !strings.Contains(r.why, "qfg-47c2.19") {
			t.Fatalf("%s: skip reason missing from why=%q", expr, r.why)
		}
		if len(r.skipped) != 1 || r.skipped[0].expr != expr {
			t.Fatalf("%s: skipped notes=%v", expr, r.skipped)
		}
	}
}

// In a compound (scenario 02), the skipped leaf is neutral and the other leaf
// is still enforced.
func TestChaosExprSkippedLeafIsNeutralInAND(t *testing.T) {
	ec := &evalCtx{probe: newChaosProbe()} // no client: connectionState() == "initializing"

	pass := evaluate("client.connectionState() == 'initializing' AND "+lagExpr, ec)
	if pass.verdict != verdictPass || len(pass.skipped) != 1 {
		t.Fatalf("AND with true leaf: verdict=%s skipped=%v, want pass with 1 skipped leaf", pass.verdict, pass.skipped)
	}

	fail := evaluate("client.connectionState() == 'connected' AND "+lagExpr, ec)
	if fail.verdict != verdictFail {
		t.Fatalf("AND with false leaf: verdict=%s why=%q, want fail", fail.verdict, fail.why)
	}

	allSkipped := evaluate(lagExpr+" AND "+lagExpr, ec)
	if allSkipped.verdict != verdictSkipped {
		t.Fatalf("AND of skipped leaves: verdict=%s, want skipped", allSkipped.verdict)
	}
}

func TestChaosExprSkippedLeafIsNeutralInOR(t *testing.T) {
	ec := &evalCtx{probe: newChaosProbe()}

	if r := evaluate("client.connectionState() == 'connected' OR "+lagExpr, ec); r.verdict != verdictFail {
		t.Fatalf("OR with false leaf: verdict=%s, want fail (skipped leaf must not satisfy OR)", r.verdict)
	}
	if r := evaluate("client.connectionState() == 'initializing' OR "+lagExpr, ec); r.verdict != verdictPass {
		t.Fatalf("OR with true leaf: verdict=%s, want pass", r.verdict)
	}
	if r := evaluate(lagExpr+" OR "+lagExpr, ec); r.verdict != verdictSkipped {
		t.Fatalf("OR of skipped leaves: verdict=%s, want skipped", r.verdict)
	}
}

// An SDK metric the probe does not implement must fail loudly, not compare
// against a silent 0.
func TestChaosExprUnknownSDKMetricFailsLoudly(t *testing.T) {
	ec := &evalCtx{probe: newChaosProbe()}
	r := evaluate("client.sdkMetric('quonfig_no_such_metric_total') == 0", ec)
	if r.verdict != verdictFail || !strings.Contains(r.why, "unknown sdkMetric") {
		t.Fatalf("verdict=%s why=%q, want fail with 'unknown sdkMetric'", r.verdict, r.why)
	}
	// Known metrics still evaluate.
	if r := evaluate("client.sdkMetric('quonfig_sse_connect_attempts_total') == 0", ec); r.verdict != verdictPass {
		t.Fatalf("known metric: verdict=%s why=%q, want pass", r.verdict, r.why)
	}
}

func TestChaosExprUnrecognizedExpressionFails(t *testing.T) {
	ec := &evalCtx{probe: newChaosProbe()}
	if r := evaluate("client.somethingNew() == 1", ec); r.verdict != verdictFail {
		t.Fatalf("verdict=%s, want fail", r.verdict)
	}
}

func TestChaosSkipTallyCountsOccurrences(t *testing.T) {
	tally := newChaosSkipTally()
	note := skipNote{expr: lagExpr, reason: serverMetricSkipReason}
	tally.record([]skipNote{note})
	tally.record([]skipNote{note})
	if got := tally.counts[lagExpr]; got != 2 {
		t.Fatalf("count=%d, want 2", got)
	}
	tally.report(t)
}
