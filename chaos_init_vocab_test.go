//go:build chaos

package quonfig

import (
	"log/slog"
	"testing"
)

// ITD v2026.10.20+ chaos vocabulary the go runner must evaluate (qfg-6vno.7):
// 00-boot-both-down-then-restore's initState/getString/readyEventCount, and
// f01/f04's case-sensitive sdkLog regex (no /i flag).

func TestChaosExprInitFailureVocabulary(t *testing.T) {
	p := newChaosProbe()
	ec := &evalCtx{probe: p}
	const getExpr = "client.getString('config.with.only.default.env.row', 'CODE-DEFAULT') == 'CODE-DEFAULT'"

	p.setInitState("pending")
	for expr, want := range map[string]exprVerdict{
		"client.initState() == 'pending'":  verdictPass,
		"client.initState() == 'returned'": verdictFail,
		getExpr:                            verdictPass, // no client yet: the caller default
		"client.readyEventCount() >= 1":    verdictFail,
		"client.readyEventCount() == 0":    verdictPass,
	} {
		if r := evaluate(expr, ec); r.verdict != want {
			t.Errorf("%s: verdict=%s (%s), want %s", expr, r.verdict, r.why, want)
		}
	}

	p.setInitState("returned")
	p.recordReadyEvent()
	if r := evaluate("client.initState() == 'returned' AND client.readyEventCount() >= 1", ec); r.verdict != verdictPass {
		t.Errorf("after return + ready event: verdict=%s (%s), want pass", r.verdict, r.why)
	}
}

func TestChaosExprSDKLogWithoutIFlagIsCaseSensitive(t *testing.T) {
	p := newChaosProbe()
	ec := &evalCtx{probe: p}
	h := &chaosLogHandler{p: p}
	logger := slog.New(h)
	logger.Warn("[quonfig] init: config not loaded yet")
	logger.Error("[QUONFIG] INIT shouting")

	cases := map[string]exprVerdict{
		`client.sdkLog('warn', /\[quonfig\] init/) == 1`:                                                     verdictPass,
		`client.sdkLog('error', /\[quonfig\] init/) == 0`:                                                    verdictPass, // case-sensitive: the shouted line does not match
		`client.sdkLog('error', /\[quonfig\] init/i) == 1`:                                                   verdictPass,
		`client.sdkLog('warn', /\[quonfig\] init/) == 0 AND client.sdkLog('error', /\[quonfig\] init/) == 0`: verdictFail,
	}
	for expr, want := range cases {
		if r := evaluate(expr, ec); r.verdict != want {
			t.Errorf("%s: verdict=%s (%s), want %s", expr, r.verdict, r.why, want)
		}
	}
}
