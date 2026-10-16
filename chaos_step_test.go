//go:build chaos

package quonfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Pure unit tests for the chaos eval-loop step and the scenario loader
// (qfg-goi1.10.32, ITD chaos/README.md "Timeline rules"). They need no rig:
// `go test -tags chaos -run 'TestChaosStep|TestChaosLoader' .`

const chaosTick = 200 * time.Millisecond

func passResult() exprResult { return exprResult{verdict: verdictPass} }
func failResult() exprResult { return exprResult{verdict: verdictFail, why: "false"} }

// drive steps one expectation from t=0 to until on the chaos tick, with
// truth(elapsed) deciding each sample. It returns the state and the elapsed
// time at which it became terminal (-1 if it never did).
func drive(exp chaosExpectation, until time.Duration, truth func(time.Duration) exprResult) (*chaosExpState, time.Duration) {
	base := time.Unix(0, 0)
	s := &chaosExpState{exp: exp}
	for el := time.Duration(0); el <= until; el += chaosTick {
		stepChaosExpectation(s, base.Add(el), el, func() exprResult { return truth(el) })
		if s.terminal() {
			return s, el
		}
	}
	return s, -1
}

// Rule 1: an expectation already true at t=0 must not pass before after_ms.
// Before the fix it latched PASS on the first tick (the sdk-java "hit at
// 276ms" evidence for 05 exp[3]).
func TestChaosStepAfterMsGatesPass(t *testing.T) {
	exp := chaosExpectation{AfterMs: 1000, WithinMs: 5000, Assert: "x"}
	s, at := drive(exp, 5000*time.Millisecond, func(time.Duration) exprResult { return passResult() })
	if !s.passed {
		t.Fatalf("want PASS, got state %+v", s)
	}
	if at < 1000*time.Millisecond || s.hitAt < 1000*time.Millisecond {
		t.Fatalf("PASS at %v (hitAt %v), want >= 1000ms (after_ms)", at, s.hitAt)
	}
}

// Before after_ms nothing happens: no hold start, no PASS, not terminal, even
// when the sample is false.
func TestChaosStepPendingBeforeAfterMs(t *testing.T) {
	exp := chaosExpectation{AfterMs: 1000, WithinMs: 5000, MustHoldForMs: 500, Assert: "x"}
	s, at := drive(exp, 800*time.Millisecond, func(time.Duration) exprResult { return passResult() })
	if at != -1 || s.terminal() || !s.heldSince.IsZero() {
		t.Fatalf("before after_ms: want pending with no hold, got at=%v state %+v", at, s)
	}
}

// The hold starts at or after after_ms and must complete by within_ms.
func TestChaosStepAfterMsHoldThenPass(t *testing.T) {
	exp := chaosExpectation{AfterMs: 1000, WithinMs: 5000, MustHoldForMs: 2000, Assert: "x"}
	s, at := drive(exp, 6000*time.Millisecond, func(time.Duration) exprResult { return passResult() })
	if !s.passed || s.hitAt != 1000*time.Millisecond || at != 3000*time.Millisecond {
		t.Fatalf("want hold from 1000ms, PASS at 3000ms; got passed=%v hitAt=%v at=%v", s.passed, s.hitAt, at)
	}
}

// Rule 3: a false sample resets the hold, and a hold that cannot complete by
// within_ms fails.
func TestChaosStepFalseSampleResetsHoldAndFailsPastWithin(t *testing.T) {
	exp := chaosExpectation{AfterMs: 1000, WithinMs: 4000, MustHoldForMs: 2000, Assert: "x"}
	truth := func(el time.Duration) exprResult {
		if el == 2600*time.Millisecond {
			return failResult()
		}
		return passResult()
	}
	s, at := drive(exp, 6000*time.Millisecond, truth)
	if s.passed || !s.failed {
		t.Fatalf("want FAIL (hold reset at 2600ms cannot finish by 4000ms), got %+v", s)
	}
	if at <= 4000*time.Millisecond {
		t.Fatalf("FAIL at %v, want past within_ms 4000ms", at)
	}
}

// Without after_ms the behavior is unchanged: PASS on the first true tick.
func TestChaosStepNoAfterMsPassesAtFirstTrue(t *testing.T) {
	s, at := drive(chaosExpectation{WithinMs: 5000, Assert: "x"}, 5000*time.Millisecond,
		func(time.Duration) exprResult { return passResult() })
	if !s.passed || at != 0 {
		t.Fatalf("want PASS at 0ms, got passed=%v at=%v", s.passed, at)
	}
}

// SKIPPED depends on shape, not time: a whole-expression server_metric row
// with after_ms still reports SKIPPED.
func TestChaosStepServerMetricWithAfterMsIsSkipped(t *testing.T) {
	exp := chaosExpectation{AfterMs: 1000, WithinMs: 5000, Assert: "server_metric('quonfig_subscriber_lag_seconds') == 0"}
	ec := &evalCtx{probe: newChaosProbe()}
	base := time.Unix(0, 0)
	s := &chaosExpState{exp: exp}
	for el := time.Duration(0); el <= 5000*time.Millisecond && !s.terminal(); el += chaosTick {
		stepChaosExpectation(s, base.Add(el), el, func() exprResult { return evaluate(exp.Assert, ec) })
	}
	if !s.skipped || s.passed || s.failed {
		t.Fatalf("want SKIPPED, got %+v", s)
	}
}

// Rule 4: the loader rejects an unknown expectation key, naming it.
func TestChaosLoaderRejectsUnknownExpectationKey(t *testing.T) {
	doc := `
function: chaos
tests:
  - name: t
    setup: {sdk: go}
    expectations:
      - within_ms: 1000
        not_before_ms: 500
        assert: "client.connectionState() == 'connected'"
`
	_, err := parseChaosScenario([]byte(doc))
	if err == nil {
		t.Fatal("want an error for unknown expectation key not_before_ms, got nil")
	}
	if !strings.Contains(err.Error(), "not_before_ms") {
		t.Fatalf("error %q does not name the key not_before_ms", err)
	}
}

func TestChaosLoaderAcceptsKnownExpectationKeys(t *testing.T) {
	doc := `
function: chaos
tests:
  - name: t
    expectations:
      - within_ms: 1000
        after_ms: 200
        must_hold_for_ms: 300
        assert: "client.connectionState() == 'connected'"
`
	s, err := parseChaosScenario([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	got := s.Tests[0].Expectations[0]
	if got.AfterMs != 200 || got.WithinMs != 1000 || got.MustHoldForMs != 300 {
		t.Fatalf("decoded %+v", got)
	}
}

// Every scenario in the pinned ITD checkout loads under the strict loader, and
// at least one carries after_ms (so the field is really being decoded).
func TestChaosLoaderParsesPinnedScenarios(t *testing.T) {
	root := filepath.Join(chaosProjectRoot(), "integration-test-data", "chaos")
	var files []string
	for _, dir := range []string{"scenarios", "scenarios-failover", "scenarios-ordering"} {
		m, _ := filepath.Glob(filepath.Join(root, dir, "*.yaml"))
		files = append(files, m...)
	}
	if len(files) == 0 {
		t.Skipf("no scenarios under %s", root)
	}
	sawAfter := false
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		s, err := parseChaosScenario(b)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		for _, run := range s.Tests {
			for _, e := range run.Expectations {
				if e.AfterMs > 0 {
					sawAfter = true
				}
			}
		}
	}
	if !sawAfter {
		t.Errorf("no scenario under %s carries after_ms; is the ITD checkout older than v2026.10.16.2?", root)
	}
}
