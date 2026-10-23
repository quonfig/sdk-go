package quonfig

import (
	"errors"
	"log/slog"
	"math"
	"strings"
	"testing"
)

// The typed-getter contract (qfg-zvyy). The cross-SDK matrix lives in
// integration-test-data (getter.contract.* fixtures); these cover what the
// corpus cannot express: values JSON cannot store, and the once-per-key log.

func typedReadClient(t *testing.T, logBuf *lockedBuf, values map[string]Value) *Client {
	t.Helper()
	opts := []Option{}
	if logBuf != nil {
		opts = append(opts, WithLogger(slog.New(slog.NewTextHandler(logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))))
	}
	client, err := NewClient(opts...)
	if err != nil {
		t.Fatal(err)
	}
	configs := make([]ConfigResponse, 0, len(values))
	for key, v := range values {
		configs = append(configs, ConfigResponse{
			Key:       key,
			Type:      ConfigTypeConfig,
			ValueType: v.Type,
			Default:   RuleSet{Rules: []Rule{{Criteria: []Criterion{{Operator: "ALWAYS_TRUE"}}, Value: v}}},
		})
	}
	client.installEnvelope(&ConfigEnvelope{Meta: Meta{Version: "v1", Environment: "Production"}, Configs: configs}, -1)
	return client
}

func TestGetIntValue_SafeDoubleConversionsOnly(t *testing.T) {
	cases := []struct {
		name string
		f    float64
		want int64
		ok   bool
	}{
		{"whole", 3.0, 3, true},
		{"negative whole", -42.0, -42, true},
		{"max safe", maxSafeInt, maxSafeInt, true},
		{"min safe", -maxSafeInt, -maxSafeInt, true},
		{"fractional", 3.9, 0, false},
		{"above safe range", maxSafeInt + 1, 0, false},
		{"below safe range", -maxSafeInt - 1, 0, false},
		{"NaN", math.NaN(), 0, false},
		{"+Inf", math.Inf(1), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := typedReadClient(t, nil, map[string]Value{"d": {Type: ValueTypeDouble, Value: tc.f}})
			got, ok, err := c.GetIntValue("d", nil)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("GetIntValue(%v) = (%v, %v, %v), want (%v, %v)", tc.f, got, ok, err, tc.want, tc.ok)
			}
			if !tc.ok && !errors.Is(err, ErrTypeMismatch) {
				t.Fatalf("err = %v, want ErrTypeMismatch", err)
			}
		})
	}
}

func TestTypedGetters_MismatchReturnsAbsentAndTypeMismatch(t *testing.T) {
	c := typedReadClient(t, nil, map[string]Value{"s": {Type: ValueTypeString, Value: "false"}})
	if got, ok, err := c.GetBoolValue("s", nil); got || ok || !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("GetBoolValue on string \"false\" = (%v, %v, %v), want (false, false, ErrTypeMismatch)", got, ok, err)
	}
	if got, ok, err := c.GetIntValue("s", nil); got != 0 || ok || !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("GetIntValue on string = (%v, %v, %v), want (0, false, ErrTypeMismatch)", got, ok, err)
	}
	if got := errorCodeFor(ErrTypeMismatch); got != ErrorCodeTypeMismatch {
		t.Fatalf("errorCodeFor(ErrTypeMismatch) = %q, want TYPE_MISMATCH", got)
	}
}

func TestTypedGetters_FailedReadLogsOncePerKey(t *testing.T) {
	var logBuf lockedBuf
	c := typedReadClient(t, &logBuf, map[string]Value{
		"a": {Type: ValueTypeString, Value: "fast"},
		"b": {Type: ValueTypeString, Value: "slow"},
	})
	for i := 0; i < 3; i++ {
		_, _, _ = c.GetBoolValue("a", nil)
		_, _, _ = c.GetIntValue("a", nil)
		_, _ = c.FeatureIsOn("a", nil)
	}
	_, _, _ = c.GetBoolValue("b", nil)
	out := logBuf.String()
	if n := countLines(out, `read of \"a\"`); n != 1 {
		t.Errorf("key a logged %d times, want 1:\n%s", n, out)
	}
	if n := countLines(out, `read of \"b\"`); n != 1 {
		t.Errorf("key b logged %d times, want 1:\n%s", n, out)
	}
	if strings.Contains(out, "fast") || strings.Contains(out, "slow") {
		t.Errorf("log leaked a value:\n%s", out)
	}
}

func TestTypedGetters_MissingKeyIsNotLogged(t *testing.T) {
	var logBuf lockedBuf
	c := typedReadClient(t, &logBuf, map[string]Value{})
	if _, ok, err := c.GetStringValue("missing", nil); ok || !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetStringValue(missing) ok=%v err=%v, want ok=false ErrNotFound", ok, err)
	}
	if out := logBuf.String(); strings.Contains(out, "missing") {
		t.Errorf("a missing key read through a typed getter must not log:\n%s", out)
	}
}

func TestFeatureIsOn_MissingKeyReturnsFalseAndLogsOnce(t *testing.T) {
	var logBuf lockedBuf
	c := typedReadClient(t, &logBuf, map[string]Value{})
	for i := 0; i < 3; i++ {
		if on, ok := c.FeatureIsOn("missing.flag", nil); on || ok {
			t.Fatalf("FeatureIsOn(missing) = (%v, %v), want (false, false)", on, ok)
		}
	}
	if n := countLines(logBuf.String(), "missing.flag"); n != 1 {
		t.Errorf("missing.flag logged %d times, want 1:\n%s", n, logBuf.String())
	}
}
