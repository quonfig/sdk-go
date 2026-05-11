//go:build chaos

// Helpers for the cross-SDK chaos harness (bead qfg-47c2.4).
//
// All chaos-test machinery is gated behind `-tags chaos` so the default
// `go test ./...` is unaffected. Run with scripts/run-chaos.sh.
//
// The shared launcher (integration-test-data/chaos/start-chaos.sh) boots
// toxiproxy and exposes chaos SSE/HTTP ports on the host. This file:
//   - loads scenario YAML
//   - drives toxiproxy via its admin API
//   - evaluates the small expression vocabulary used in the scenarios
//   - tracks SDK state via the probe struct
//
// The probe is intentionally a thin wrapper over the SDK's existing public
// callbacks (OnSSEStateChange, OnConfigUpdate, slog handler). It does NOT
// invent metrics the SDK does not emit — that is the whole point of this
// bead: the missing surface area (worker_restart_total, fallbackPollerActive,
// lastSuccessfulRefresh, …) is what makes scenarios 2/5/7/9 fail today.

package quonfig

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// ----- scenario YAML -----

type chaosScenario struct {
	Function string             `yaml:"function"`
	Tests    []chaosScenarioRun `yaml:"tests"`
}

type chaosScenarioRun struct {
	Name         string             `yaml:"name"`
	Description  string             `yaml:"description"`
	Setup        chaosSetup         `yaml:"setup"`
	Chaos        []chaosEvent       `yaml:"chaos"`
	Expectations []chaosExpectation `yaml:"expectations"`
}

type chaosSetup struct {
	SDK              string `yaml:"sdk"`
	SSEEndpoint      string `yaml:"sse_endpoint"`
	HTTPEndpoint     string `yaml:"http_endpoint"`
	WallClockSeconds int    `yaml:"wall_clock_seconds"`
	UserCallback     string `yaml:"user_callback"`
}

type chaosEvent struct {
	AtMs    int          `yaml:"at_ms"`
	Inject  *chaosInject `yaml:"inject,omitempty"`
	Clear   string       `yaml:"clear,omitempty"`
	Process *chaosProc   `yaml:"process,omitempty"`
}

type chaosInject struct {
	Name string `yaml:"name"`

	// Convenience aliases.
	SSESilentStallAfterMs *int `yaml:"sse_silent_stall_after_ms"`
	SSELatencyMs          *int `yaml:"sse_latency_ms"`
	SSEBandwidthKbps      *int `yaml:"sse_bandwidth_kbps"`
	SSEDownMs             *int `yaml:"sse_down_ms"`
	BothDownMs            *int `yaml:"both_down_ms"`
	SSEHalfOpenAfterBytes *int `yaml:"sse_half_open_after_bytes"`
	SSEHTTPStatus         *int `yaml:"sse_http_status"`

	// Low-level escape hatch (not used in any current scenario but kept for completeness).
	Proxy string                 `yaml:"proxy"`
	Toxic map[string]interface{} `yaml:"toxic"`
}

type chaosProc struct {
	Action     string `yaml:"action"`
	Count      int    `yaml:"count"`
	IntervalMs int    `yaml:"interval_ms"`
}

type chaosExpectation struct {
	WithinMs      int    `yaml:"within_ms"`
	MustHoldForMs int    `yaml:"must_hold_for_ms"`
	Assert        string `yaml:"assert"`
}

func loadChaosScenario(t *testing.T, path string) chaosScenario {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read scenario %s: %v", path, err)
	}
	var s chaosScenario
	if err := yaml.Unmarshal(b, &s); err != nil {
		t.Fatalf("parse scenario %s: %v", path, err)
	}
	return s
}

// ----- toxiproxy admin client -----

type toxiproxyClient struct {
	base string
	hc   *http.Client
}

func newToxiproxy(base string) *toxiproxyClient {
	return &toxiproxyClient{base: strings.TrimRight(base, "/"), hc: &http.Client{Timeout: 5 * time.Second}}
}

func (tp *toxiproxyClient) ping(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", tp.base+"/version", nil)
	resp, err := tp.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("toxiproxy /version: %s", resp.Status)
	}
	return nil
}

// upsertProxy creates the named proxy (deleting any existing) pointing at the
// given listen/upstream addresses. Used to reconfigure proxies seeded by
// start-chaos.sh so they target our locally-spawned api-delivery.
func (tp *toxiproxyClient) upsertProxy(t *testing.T, name, listen, upstream string) {
	t.Helper()
	// Delete (ignore 404)
	req, _ := http.NewRequest("DELETE", tp.base+"/proxies/"+name, nil)
	resp, err := tp.hc.Do(req)
	if err == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	body := map[string]interface{}{
		"name":     name,
		"listen":   listen,
		"upstream": upstream,
		"enabled":  true,
	}
	bs, _ := json.Marshal(body)
	resp, err = tp.hc.Post(tp.base+"/proxies", "application/json", bytes.NewReader(bs))
	if err != nil {
		t.Fatalf("create proxy %s: %v", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("create proxy %s: %s — %s", name, resp.Status, out)
	}
}

func (tp *toxiproxyClient) clearToxics(t *testing.T, proxy string) {
	t.Helper()
	req, _ := http.NewRequest("GET", tp.base+"/proxies/"+proxy+"/toxics", nil)
	resp, err := tp.hc.Do(req)
	if err != nil {
		t.Fatalf("list toxics on %s: %v", proxy, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return
	}
	var toxics []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&toxics); err != nil {
		t.Fatalf("decode toxics on %s: %v", proxy, err)
	}
	for _, tox := range toxics {
		nm, _ := tox["name"].(string)
		if nm == "" {
			continue
		}
		dreq, _ := http.NewRequest("DELETE", tp.base+"/proxies/"+proxy+"/toxics/"+nm, nil)
		dresp, derr := tp.hc.Do(dreq)
		if derr == nil {
			_, _ = io.Copy(io.Discard, dresp.Body)
			dresp.Body.Close()
		}
	}
}

func (tp *toxiproxyClient) setEnabled(t *testing.T, proxy string, enabled bool) {
	t.Helper()
	body := map[string]interface{}{"enabled": enabled}
	bs, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", tp.base+"/proxies/"+proxy, bytes.NewReader(bs))
	req.Header.Set("Content-Type", "application/json")
	resp, err := tp.hc.Do(req)
	if err != nil {
		t.Fatalf("set proxy %s enabled=%v: %v", proxy, enabled, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("set proxy %s enabled=%v: %s — %s", proxy, enabled, resp.Status, out)
	}
}

// addToxic posts a toxic to the named proxy. attrs is the toxic-specific
// attribute map (e.g. {"timeout": 0} or {"latency": 5000}).
func (tp *toxiproxyClient) addToxic(t *testing.T, proxy, name, toxicType, stream string, attrs map[string]interface{}) {
	t.Helper()
	if stream == "" {
		stream = "downstream"
	}
	body := map[string]interface{}{
		"name":       name,
		"type":       toxicType,
		"stream":     stream,
		"attributes": attrs,
	}
	bs, _ := json.Marshal(body)
	resp, err := tp.hc.Post(tp.base+"/proxies/"+proxy+"/toxics", "application/json", bytes.NewReader(bs))
	if err != nil {
		t.Fatalf("add toxic %s/%s: %v", proxy, name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("add toxic %s/%s: %s — %s", proxy, name, resp.Status, out)
	}
}

func (tp *toxiproxyClient) removeToxic(t *testing.T, proxy, name string) {
	t.Helper()
	req, _ := http.NewRequest("DELETE", tp.base+"/proxies/"+proxy+"/toxics/"+name, nil)
	resp, err := tp.hc.Do(req)
	if err != nil {
		t.Fatalf("delete toxic %s/%s: %v", proxy, name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 && resp.StatusCode != 404 {
		out, _ := io.ReadAll(resp.Body)
		t.Fatalf("delete toxic %s/%s: %s — %s", proxy, name, resp.Status, out)
	}
}

// ----- chaos injection plan -----

// injectionState lets `clear:` find which proxy and toxic name were created by
// a prior `inject:` block under the same logical name.
type injectionState struct {
	proxy  string
	toxic  string
	enable []string // proxies to re-enable on clear (for sse_down / both_down)
}

// applyInject performs the convenience-alias translation for one inject event.
// Returns the injectionState so a later `clear:` can undo it.
func applyInject(t *testing.T, tp *toxiproxyClient, inj *chaosInject) *injectionState {
	t.Helper()
	if inj == nil {
		return nil
	}
	name := inj.Name
	if name == "" {
		name = "anon"
	}
	switch {
	case inj.SSESilentStallAfterMs != nil:
		// Toxiproxy "timeout" with timeout=0 stops all data from getting
		// through and keeps the connection open until the toxic is cleared —
		// exactly the silent-stall semantic.
		tp.addToxic(t, "sse", name, "timeout", "downstream", map[string]interface{}{
			"timeout": *inj.SSESilentStallAfterMs,
		})
		return &injectionState{proxy: "sse", toxic: name}
	case inj.SSELatencyMs != nil:
		tp.addToxic(t, "sse", name, "latency", "downstream", map[string]interface{}{
			"latency": *inj.SSELatencyMs,
		})
		return &injectionState{proxy: "sse", toxic: name}
	case inj.SSEBandwidthKbps != nil:
		tp.addToxic(t, "sse", name, "bandwidth", "downstream", map[string]interface{}{
			"rate": *inj.SSEBandwidthKbps,
		})
		return &injectionState{proxy: "sse", toxic: name}
	case inj.SSEDownMs != nil:
		tp.setEnabled(t, "sse", false)
		return &injectionState{enable: []string{"sse"}}
	case inj.BothDownMs != nil:
		tp.setEnabled(t, "sse", false)
		tp.setEnabled(t, "http", false)
		return &injectionState{enable: []string{"sse", "http"}}
	case inj.SSEHalfOpenAfterBytes != nil:
		tp.addToxic(t, "sse", name, "limit_data", "downstream", map[string]interface{}{
			"bytes": *inj.SSEHalfOpenAfterBytes,
		})
		return &injectionState{proxy: "sse", toxic: name}
	case inj.SSEHTTPStatus != nil:
		// HTTP-level status injection is not toxiproxy-native (toxiproxy is
		// TCP-only). Skipping this case turns the scenario into a no-op —
		// the runner records it as `skipped: not supported`.
		t.Logf("inject: sse_http_status=%d — toxiproxy is TCP-only, not implemented (scenario will be skipped/unsupported)", *inj.SSEHTTPStatus)
		return &injectionState{}
	case inj.Proxy != "" && inj.Toxic != nil:
		toxicType, _ := inj.Toxic["type"].(string)
		attrs, _ := inj.Toxic["attributes"].(map[string]interface{})
		if attrs == nil {
			attrs = map[string]interface{}{}
		}
		tp.addToxic(t, inj.Proxy, name, toxicType, "downstream", attrs)
		return &injectionState{proxy: inj.Proxy, toxic: name}
	}
	t.Logf("inject: unknown shape (%+v) — no-op", inj)
	return nil
}

func clearInject(t *testing.T, tp *toxiproxyClient, st *injectionState) {
	if st == nil {
		return
	}
	if st.toxic != "" {
		tp.removeToxic(t, st.proxy, st.toxic)
	}
	for _, p := range st.enable {
		tp.setEnabled(t, p, true)
	}
}

func applyProcess(t *testing.T, tp *toxiproxyClient, p *chaosProc, baseline time.Time) {
	t.Helper()
	if p == nil {
		return
	}
	switch p.Action {
	case "kill_sse_proxy":
		// Toggle enabled to forcibly close all open connections, briefly,
		// `count` times with `interval_ms` between each pair.
		count := p.Count
		if count <= 0 {
			count = 1
		}
		interval := time.Duration(p.IntervalMs) * time.Millisecond
		if interval <= 0 {
			interval = 1 * time.Second
		}
		go func() {
			for i := 0; i < count; i++ {
				tp.setEnabled(t, "sse", false)
				// Brief downtime so existing connections actually drop.
				time.Sleep(200 * time.Millisecond)
				tp.setEnabled(t, "sse", true)
				if i < count-1 {
					time.Sleep(interval - 200*time.Millisecond)
				}
			}
		}()
	default:
		t.Logf("process: unknown action %q — no-op", p.Action)
	}
	_ = baseline
}

// ----- SDK probe -----

// chaosProbe wraps an sdk-go Client with the test-time observation surface
// the scenarios reference. It does NOT add metrics to the SDK itself —
// instead it derives approximations from existing callbacks. Missing surface
// area (worker_restart_total, fallbackPollerActive, lastSuccessfulRefresh,
// connect_attempts_total) is the *point* of this harness: the scenarios fail
// because the SDK does not yet emit those signals.
type chaosProbe struct {
	client *Client

	mu             sync.Mutex
	connState      string // initializing | connected | reconnecting | falling_back | disconnected
	lastRefresh    time.Time
	connAttempts   int64 // count of (any) state transition into connected
	restartLayer1  int64 // count of (connected → disconnected) edges; sdk-go never restarts on stall today so this stays 0 there
	restartLayer2  int64 // never increments — no Layer 2 exists
	fallbackActive bool  // always false today
	processCrashed atomic.Bool
	logBuf         bytes.Buffer
	logMu          sync.Mutex
}

func newChaosProbe() *chaosProbe {
	return &chaosProbe{connState: "initializing"}
}

func (p *chaosProbe) connectionState() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.connState
}

func (p *chaosProbe) sdkMetric(name string, labels map[string]string) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch name {
	case "quonfig_sdk_worker_restart_total":
		switch labels["layer"] {
		case "1":
			return float64(p.restartLayer1)
		case "2":
			return float64(p.restartLayer2)
		default:
			return float64(p.restartLayer1 + p.restartLayer2)
		}
	case "quonfig_sse_connect_attempts_total":
		return float64(p.connAttempts)
	}
	return 0
}

func (p *chaosProbe) fallbackPollerActive() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fallbackActive
}

func (p *chaosProbe) lastSuccessfulRefreshMs() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastRefresh.IsZero() {
		return 0
	}
	return p.lastRefresh.UnixMilli()
}

func (p *chaosProbe) processStillAlive() bool {
	return !p.processCrashed.Load()
}

func (p *chaosProbe) sdkLogMatches(level string, re *regexp.Regexp) int {
	p.logMu.Lock()
	defer p.logMu.Unlock()
	out := 0
	for _, line := range strings.Split(p.logBuf.String(), "\n") {
		if line == "" {
			continue
		}
		// Best-effort: the slog text handler emits `level=ERROR`.
		if level != "" && !strings.Contains(strings.ToLower(line), "level="+strings.ToLower(level)) {
			continue
		}
		if re == nil || re.MatchString(line) {
			out++
		}
	}
	return out
}

func (p *chaosProbe) onSSEState(connected bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case connected:
		// transition into connected
		p.connState = "connected"
		p.connAttempts++
	default:
		// transition out — sdk-go's loop always retries until Stop, so map
		// false → "reconnecting" rather than "disconnected".
		p.connState = "reconnecting"
	}
	// NB: we do NOT increment restartLayer1 from state transitions.
	// `quonfig_sdk_worker_restart_total` is a *supervisor*-restart counter
	// (panic-in-callback recovery, deadline-trip-driven worker re-spawn) per
	// the plan's worker-restart vocabulary. The natural reconnect cycle in
	// today's sdk-go does not flow through any supervisor, so the metric
	// stays at 0. That is precisely what makes scenarios 2/7/9 fail today
	// and what B4/B5/B6 will turn green by wiring a real supervisor.
}

func (p *chaosProbe) onConfigUpdate() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastRefresh = time.Now()
}

// slogHandler collects everything the SDK logs into logBuf so sdkLog can scan.
type chaosLogHandler struct {
	p *chaosProbe
}

func (h *chaosLogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *chaosLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.p.logMu.Lock()
	defer h.p.logMu.Unlock()
	fmt.Fprintf(&h.p.logBuf, "level=%s msg=%q", r.Level.String(), r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&h.p.logBuf, " %s=%v", a.Key, a.Value)
		return true
	})
	h.p.logBuf.WriteByte('\n')
	return nil
}

func (h *chaosLogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *chaosLogHandler) WithGroup(_ string) slog.Handler      { return h }

// ----- expression evaluator -----
//
// The scenarios use a small expression vocabulary. We do not need a full
// parser; we recognize a fixed set of leaf shapes and combine with the binary
// connectives AND/OR.

type evalCtx struct {
	probe *chaosProbe
	// server-side metrics are not yet implemented (server-side highwater work
	// lives in a separate bead). Stub to 0 so scenarios that reference
	// server_metric do not synthetically fail before SDK-side red proof.
	serverMetric func(name string) float64
}

// evaluate parses and evaluates one assert string against the current state.
// Returns (true, "") on hit, (false, reason) on miss. The reason string is for
// debug logging only.
func evaluate(expr string, ec *evalCtx) (bool, string) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return true, ""
	}
	// Top-level: split on " OR " (lowest precedence). Each OR-leaf is then
	// split on " AND " (higher precedence). Parentheses are not used by any
	// scenario shipped to date.
	if strings.Contains(expr, " OR ") {
		parts := splitOutsideQuotesAndRegex(expr, " OR ")
		var reasons []string
		for _, p := range parts {
			ok, why := evaluate(p, ec)
			if ok {
				return true, ""
			}
			reasons = append(reasons, why)
		}
		return false, "OR: " + strings.Join(reasons, " | ")
	}
	if strings.Contains(expr, " AND ") {
		parts := splitOutsideQuotesAndRegex(expr, " AND ")
		for _, p := range parts {
			ok, why := evaluate(p, ec)
			if !ok {
				return false, "AND: " + why
			}
		}
		return true, ""
	}
	return evalLeaf(expr, ec)
}

// splitOutsideQuotesAndRegex splits expr on sep, but ignores occurrences that
// fall inside a single-quoted string or a `/regex/i` literal. The scenario
// vocabulary uses both kinds of literals so a naive strings.Split is unsafe.
func splitOutsideQuotesAndRegex(expr, sep string) []string {
	var out []string
	depthSQ := false
	depthRE := false
	start := 0
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		switch c {
		case '\'':
			if !depthRE {
				depthSQ = !depthSQ
			}
		case '/':
			if !depthSQ {
				depthRE = !depthRE
			}
		}
		if !depthSQ && !depthRE && i+len(sep) <= len(expr) && expr[i:i+len(sep)] == sep {
			out = append(out, expr[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	out = append(out, expr[start:])
	return out
}

var (
	reConnStateEq = regexp.MustCompile(`^client\.connectionState\(\)\s*(==|!=)\s*'([^']+)'$`)
	reFallbackEq  = regexp.MustCompile(`^client\.fallbackPollerActive\(\)\s*==\s*(true|false)$`)
	reProcAliveEq = regexp.MustCompile(`^client\.processStillAlive\(\)\s*==\s*(true|false)$`)
	reLastRefresh = regexp.MustCompile(`^client\.lastSuccessfulRefresh\(\)\s*(>=|>|<=|<|==)\s*\(now\(\)\s*-\s*(\d+)\)$`)
	reSDKMetric   = regexp.MustCompile(`^client\.sdkMetric\(\s*'([^']+)'\s*(?:,\s*layer=\s*'([^']+)'\s*)?\)\s*(>=|<=|==|!=|<|>)\s*(\d+)$`)
	reServerMet   = regexp.MustCompile(`^server_metric\(\s*'([^']+)'\s*\)\s*(>=|<=|==|!=|<|>)\s*(\d+)$`)
	reSDKLog      = regexp.MustCompile(`^client\.sdkLog\(\s*'([^']+)'\s*,\s*/(.+)/i\s*\)\s*(>=|<=|==|!=|<|>)\s*(\d+)$`)
)

func evalLeaf(expr string, ec *evalCtx) (bool, string) {
	expr = strings.TrimSpace(expr)
	if m := reConnStateEq.FindStringSubmatch(expr); m != nil {
		got := ec.probe.connectionState()
		want := m[2]
		switch m[1] {
		case "==":
			ok := got == want
			return ok, fmt.Sprintf("connectionState=%s want %s", got, want)
		case "!=":
			ok := got != want
			return ok, fmt.Sprintf("connectionState=%s want != %s", got, want)
		}
	}
	if m := reFallbackEq.FindStringSubmatch(expr); m != nil {
		want := m[1] == "true"
		got := ec.probe.fallbackPollerActive()
		return got == want, fmt.Sprintf("fallbackPollerActive=%v want %v", got, want)
	}
	if m := reProcAliveEq.FindStringSubmatch(expr); m != nil {
		want := m[1] == "true"
		got := ec.probe.processStillAlive()
		return got == want, fmt.Sprintf("processStillAlive=%v want %v", got, want)
	}
	if m := reLastRefresh.FindStringSubmatch(expr); m != nil {
		ago, _ := strconv.Atoi(m[2])
		last := ec.probe.lastSuccessfulRefreshMs()
		threshold := time.Now().UnixMilli() - int64(ago)
		ok := compareInt(m[1], last, threshold)
		return ok, fmt.Sprintf("lastSuccessfulRefresh=%d %s (now()-%d)=%d", last, m[1], ago, threshold)
	}
	if m := reSDKMetric.FindStringSubmatch(expr); m != nil {
		labels := map[string]string{}
		if m[2] != "" {
			labels["layer"] = m[2]
		}
		got := ec.probe.sdkMetric(m[1], labels)
		want, _ := strconv.ParseFloat(m[4], 64)
		ok := compareFloat(m[3], got, want)
		return ok, fmt.Sprintf("sdkMetric(%s,layer=%s)=%v %s %v", m[1], m[2], got, m[3], want)
	}
	if m := reServerMet.FindStringSubmatch(expr); m != nil {
		got := ec.serverMetric(m[1])
		want, _ := strconv.ParseFloat(m[3], 64)
		ok := compareFloat(m[2], got, want)
		return ok, fmt.Sprintf("server_metric(%s)=%v %s %v", m[1], got, m[2], want)
	}
	if m := reSDKLog.FindStringSubmatch(expr); m != nil {
		level := m[1]
		re, err := regexp.Compile("(?i)" + m[2])
		if err != nil {
			return false, fmt.Sprintf("bad regex %q: %v", m[2], err)
		}
		n := ec.probe.sdkLogMatches(level, re)
		want, _ := strconv.Atoi(m[4])
		ok := compareInt(m[3], int64(n), int64(want))
		return ok, fmt.Sprintf("sdkLog(%s,/%s/i)=%d %s %d", level, m[2], n, m[3], want)
	}
	return false, fmt.Sprintf("unrecognized expression: %s", expr)
}

func compareInt(op string, a, b int64) bool {
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return false
}

func compareFloat(op string, a, b float64) bool {
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return false
}

// ----- monorepo path helpers (mirror integration_test.go style) -----

func chaosProjectRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..")
}

func chaosScenariosDir() string {
	return filepath.Join(chaosProjectRoot(), "integration-test-data", "chaos", "scenarios")
}

func chaosIntegrationDataDir() string {
	return filepath.Join(chaosProjectRoot(), "integration-test-data", "data", "integration-tests")
}

func chaosFixtureKeysPath() string {
	return filepath.Join(chaosProjectRoot(), "api-delivery", "testdata", "fixture-sdk-keys.json")
}
