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
// The probe is a thin reader over the SDK: every state and metric it reports
// comes from the Client's accessors or its internal counters (workerMetrics,
// the supervisor's restart counter), never from the probe's own bookkeeping
// (qfg-6vno.1). The only thing it records itself is the log text sdkLog
// scans, which is the SDK's own output.

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
	SDK              string          `yaml:"sdk"`
	SSEEndpoint      string          `yaml:"sse_endpoint"`
	HTTPEndpoint     string          `yaml:"http_endpoint"`
	WallClockSeconds int             `yaml:"wall_clock_seconds"`
	UserCallback     string          `yaml:"user_callback"`
	Topology         string          `yaml:"topology"`  // "", single, failover, ordering
	Upstreams        []chaosUpstream `yaml:"upstreams"` // ordering rig: per-leg generations

	// Init-failure scenarios (00-boot-both-down-then-restore).
	OnInitFailure    string `yaml:"on_init_failure"` // fail | wait | continue
	InitTimeoutMs    int    `yaml:"init_timeout_ms"`
	CatchInitFailure bool   `yaml:"catch_init_failure"`
}

// chaosUpstream is one leg of the ordering rig — a fixture upstream pinned to a
// chosen Meta.generation so o01-o04 can stand up primary and secondary at
// divergent generations.
type chaosUpstream struct {
	Role       string `yaml:"role"` // primary | secondary
	Generation int    `yaml:"generation"`
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

	// Failover-rig aliases (scenarios-failover/): fault the PRIMARY HTTP leg
	// only. Each carries a duration in ms after which the fault auto-clears, so
	// the failover scenarios need no explicit `clear` event (see
	// applyFailoverInject).
	PrimaryRefusedMs *int `yaml:"primary_refused_ms"` // disable the primary proxy (port refuses)
	PrimaryHangMs    *int `yaml:"primary_hang_ms"`    // timeout toxic: accept but never respond
	PrimaryLatencyMs *int `yaml:"primary_latency_ms"` // latency toxic: respond, but far too late

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
	AfterMs       int    `yaml:"after_ms"`
	Assert        string `yaml:"assert"`
}

func loadChaosScenario(t *testing.T, path string) chaosScenario {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read scenario %s: %v", path, err)
	}
	s, err := parseChaosScenario(b)
	if err != nil {
		t.Fatalf("parse scenario %s: %v", path, err)
	}
	return s
}

func parseChaosScenario(b []byte) (chaosScenario, error) {
	var s chaosScenario
	err := yaml.Unmarshal(b, &s)
	return s, err
}

// chaosExpectationKeys is the closed key set of a chaos expectation (ITD
// chaos/README.md, Timeline rule 4; the schema's additionalProperties: false).
var chaosExpectationKeys = map[string]bool{
	"within_ms": true, "assert": true, "must_hold_for_ms": true, "after_ms": true,
}

// UnmarshalYAML rejects any expectation key outside chaosExpectationKeys,
// naming it, so a harness pinned to a newer ITD than it implements fails the
// run instead of silently dropping a timing field such as after_ms.
func (e *chaosExpectation) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i]
			if !chaosExpectationKeys[k.Value] {
				return fmt.Errorf("line %d: unknown chaos expectation key %q (allowed: within_ms, assert, must_hold_for_ms, after_ms)", k.Line, k.Value)
			}
		}
	}
	type plain chaosExpectation // no UnmarshalYAML method: avoids recursion
	return node.Decode((*plain)(e))
}

// ----- per-tick expectation step -----

// chaosExpState is one expectation's progress through the eval loop. Both
// loops (runChaosScenario and evalRigExpectations) drive it with
// stepChaosExpectation.
type chaosExpState struct {
	exp        chaosExpectation
	hitAt      time.Duration
	heldSince  time.Time
	passed     bool
	failed     bool // within_ms elapsed without pass
	skipped    bool // the rig cannot evaluate it; see serverMetricSkipReason
	lastReason string
	skipNotes  []skipNote
}

func (s *chaosExpState) terminal() bool { return s.passed || s.failed || s.skipped }

// stepChaosExpectation advances one non-terminal expectation by one tick.
// now/elapsed are the tick time and its offset from t=0; eval evaluates the
// expectation's assert against the current probe state.
func stepChaosExpectation(s *chaosExpState, now time.Time, elapsed time.Duration, eval func() exprResult) {
	if s.terminal() {
		return
	}
	// Timeline rule 1: before after_ms the expectation is not evaluated: no
	// PASS, no hold start, not terminal. SKIPPED is a property of the
	// expression's shape (a whole-expression server_metric), not of time, so
	// a skipped-shape row is still reported SKIPPED; that is the only result
	// honored before after_ms.
	if elapsed < time.Duration(s.exp.AfterMs)*time.Millisecond {
		if r := eval(); r.verdict == verdictSkipped {
			s.lastReason = r.why
			s.skipNotes = r.skipped
			s.skipped = true
		}
		return
	}
	r := eval()
	s.lastReason = r.why
	s.skipNotes = r.skipped
	if r.verdict == verdictSkipped {
		s.skipped = true
		return
	}
	if r.verdict == verdictPass {
		if s.heldSince.IsZero() {
			s.heldSince = now
			s.hitAt = elapsed
		}
		holdFor := time.Duration(s.exp.MustHoldForMs) * time.Millisecond
		if holdFor <= 0 || now.Sub(s.heldSince) >= holdFor {
			s.passed = true
		}
	} else {
		s.heldSince = time.Time{}
	}
	// Definitive fail: within_ms has elapsed and we either never hit, or we
	// hit but never held long enough. Hold-for failures still have a chance
	// until within_ms; after that it's terminal.
	if !s.passed && elapsed > time.Duration(s.exp.WithinMs)*time.Millisecond {
		s.failed = true
	}
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
		// Toxiproxy is TCP-only and can't truly model "server returns 200 then
		// closes after N bytes" — the limit_data toxic this used to call only
		// trips on the NEXT upstream byte, which for SSE is the 30s heartbeat,
		// outside the typical within_ms=15s window. The closest TCP-only
		// analog is to disable the proxy: existing SSE connections drop, new
		// attempts are refused. Leave it disabled until the matching `clear`
		// step so the SDK's reconnect attempts fail visibly (sdk-ruby's
		// ld-eventsource only fires on_error on ECONNREFUSED, not on clean
		// FIN). qfg-47c2.29.
		tp.setEnabled(t, "sse", false)
		return &injectionState{enable: []string{"sse"}}
	case inj.SSEHTTPStatus != nil:
		// HTTP-level status injection is not toxiproxy-native (toxiproxy is
		// TCP-only). Skipping this case turns the scenario into a no-op —
		// the runner records it as `skipped: not supported`.
		// validateChaosEvents SKIPs a scenario carrying it before it starts, so
		// reaching here is a rig bug.
		t.Errorf("inject: sse_http_status=%d — toxiproxy is TCP-only and cannot apply it", *inj.SSEHTTPStatus)
		return nil
	case inj.Proxy != "" && inj.Toxic != nil:
		toxicType, _ := inj.Toxic["type"].(string)
		attrs, _ := inj.Toxic["attributes"].(map[string]interface{})
		if attrs == nil {
			attrs = map[string]interface{}{}
		}
		tp.addToxic(t, inj.Proxy, name, toxicType, "downstream", attrs)
		return &injectionState{proxy: inj.Proxy, toxic: name}
	}
	t.Errorf("inject: unknown shape (%+v) — the rig cannot apply it", inj)
	return nil
}

// validateChaosEvents checks, before a scenario starts, that this rig can
// perform every chaos event in it. An event it cannot perform must never turn
// into a silent no-op: the expectations would then run against an
// undisturbed client and pass. It returns an error for an event shape the rig
// does not know (the scenario fails), or a non-empty skip reason for a known
// shape this TCP-only rig deliberately cannot apply (the scenario is SKIPPED,
// never PASS).
func validateChaosEvents(events []chaosEvent) (skipReason string, err error) {
	injected := map[string]bool{}
	for i, ev := range events {
		switch {
		case ev.Inject != nil:
			inj := ev.Inject
			switch {
			case inj.SSEHTTPStatus != nil:
				if skipReason == "" {
					skipReason = "sse_http_status needs the HTTP-aware fault proxy (scenarios-http-proxy/); toxiproxy is TCP-only"
				}
			case inj.SSESilentStallAfterMs != nil, inj.SSELatencyMs != nil, inj.SSEBandwidthKbps != nil,
				inj.SSEDownMs != nil, inj.BothDownMs != nil, inj.SSEHalfOpenAfterBytes != nil,
				inj.Proxy != "" && inj.Toxic != nil:
			default:
				return "", fmt.Errorf("chaos[%d]: inject shape %+v is not supported by this rig", i, *inj)
			}
			if inj.Name != "" {
				injected[inj.Name] = true
			}
		case ev.Clear != "":
			if !injected[ev.Clear] {
				return "", fmt.Errorf("chaos[%d]: clear %q names no earlier inject", i, ev.Clear)
			}
		case ev.Process != nil:
			if ev.Process.Action != "kill_sse_proxy" {
				return "", fmt.Errorf("chaos[%d]: process action %q is not supported by this rig", i, ev.Process.Action)
			}
		default:
			return "", fmt.Errorf("chaos[%d]: event has no inject, clear or process", i)
		}
	}
	return skipReason, nil
}

// validateFailoverChaosEvents is validateChaosEvents for the failover rig,
// which applies only its self-restoring inject aliases (applyFailoverInject)
// and has no clear or process handling.
func validateFailoverChaosEvents(events []chaosEvent) error {
	for i, ev := range events {
		inj := ev.Inject
		if inj == nil || ev.Clear != "" || ev.Process != nil {
			return fmt.Errorf("chaos[%d]: the failover rig handles only inject events (primary_refused_ms, primary_hang_ms, primary_latency_ms, sse_down_ms); got %+v", i, ev)
		}
		switch {
		case inj.PrimaryRefusedMs != nil, inj.PrimaryHangMs != nil, inj.PrimaryLatencyMs != nil, inj.SSEDownMs != nil:
		default:
			return fmt.Errorf("chaos[%d]: inject shape %+v is not supported by the failover rig", i, *inj)
		}
	}
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
		t.Errorf("process: unknown action %q — the rig cannot apply it", p.Action)
	}
	_ = baseline
}

// ----- SDK probe -----

// chaosProbe wraps an sdk-go Client with the test-time observation surface
// the scenarios reference. Every value it reports is read from the SDK (see
// the file header); the probe keeps no counters of its own.
type chaosProbe struct {
	client *Client

	mu sync.Mutex
	// initStatus backs client.initState() for init-failure scenarios:
	// "pending" while NewClient has not returned, then "returned" or
	// "raised". It records what the init call did; it is not an SDK metric.
	initStatus string
	// initReturned gates readyEvents: readyEventCount() counts the SDK's
	// OnConfigUpdate callbacks fired after the init call returned.
	initReturned bool
	readyEvents  int64

	logBuf bytes.Buffer
	logMu  sync.Mutex
}

func (p *chaosProbe) setInitState(s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.initStatus = s
	if s == "returned" || s == "raised" {
		p.initReturned = true
	}
}

func (p *chaosProbe) initState() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.initStatus
}

// recordReadyEvent is the OnConfigUpdate hook: one SDK-fired update event,
// counted only once the init call has returned.
func (p *chaosProbe) recordReadyEvent() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.initReturned {
		p.readyEvents++
	}
}

func (p *chaosProbe) readyEventCount() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.readyEvents
}

// getString is the SDK's public string getter with a caller default: the live
// value, or def when the SDK has no value for the key (including before the
// client exists).
func (p *chaosProbe) getString(key, def string) string {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	if c == nil {
		return def
	}
	v, ok, err := c.GetStringValue(key, nil)
	if err != nil || !ok {
		return def
	}
	return v
}

// chaosProbeClientOptions are the options every chaos rig passes so the probe
// can observe the client: the SDK's log output, for sdkLog.
func chaosProbeClientOptions(p *chaosProbe) []Option {
	return []Option{
		WithLogger(slog.New(&chaosLogHandler{p: p})),
	}
}

func newChaosProbe() *chaosProbe {
	return &chaosProbe{}
}

func (p *chaosProbe) setClient(c *Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.client = c
}

func (p *chaosProbe) connectionState() string {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	if c == nil {
		return "initializing"
	}
	state := c.ConnectionState()
	switch state {
	case ConnStateInitializing, ConnStateConnected, ConnStateDisconnected, ConnStateFallingBack:
		return string(state)
	}
	// The supervisor contract fixes these four values; report anything else
	// so no expectation can match it.
	return "invalid:" + string(state)
}

// sdkMetric returns the probe's value for an SDK-side metric. known is false
// for a metric name the probe does not implement, so the evaluator fails the
// expectation loudly instead of comparing against a silent 0.
//
// Sources: Layer 1 restarts and connect attempts are the Client's internal
// workerMetrics (counted in sseClient.runLoop/connectOnce); callback errors
// are the same workerMetrics (counted in the callback recover guards);
// Layer 2 restarts are the supervisor's
// WorkerRestartTotal("2"). Before the client exists every count is 0.
func (p *chaosProbe) sdkMetric(name string, labels map[string]string) (value float64, known bool) {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	layer1 := func() int64 { return 0 }
	layer2 := func() int64 { return 0 }
	attempts := func() int64 { return 0 }
	if c != nil {
		c.mu.RLock()
		sup := c.sup
		c.mu.RUnlock()
		layer1 = func() int64 {
			n := c.metrics.restartTotal("")
			if sup != nil {
				n += sup.WorkerRestartTotal("1")
			}
			return n
		}
		layer2 = func() int64 {
			if sup == nil {
				return 0
			}
			return sup.WorkerRestartTotal("2")
		}
		attempts = c.metrics.sseConnectAttempts.Load
	}
	switch name {
	case "quonfig_sdk_worker_restart_total":
		switch labels["layer"] {
		case "1":
			return float64(layer1()), true
		case "2":
			return float64(layer2()), true
		case "":
			return float64(layer1() + layer2()), true
		}
		return 0, false
	case "quonfig_sse_connect_attempts_total":
		return float64(attempts()), true
	case "quonfig_sdk_callback_errors_total":
		// Callback panics recovered by the SDK (contract Test 5,
		// qfg-6vno.26); "" sums every callback.
		if c == nil {
			return 0, true
		}
		return float64(c.metrics.callbackErrorTotal(labels["callback"])), true
	}
	return 0, false
}

func (p *chaosProbe) fallbackPollerActive() bool {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	if c == nil {
		return false
	}
	return c.FallbackPollerActive()
}

func (p *chaosProbe) lastSuccessfulRefreshMs() int64 {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	if c == nil {
		return 0
	}
	last := c.LastSuccessfulRefresh()
	if last.IsZero() {
		return 0
	}
	return last.UnixMilli()
}

// processStillAlive is true by construction, and that is a real observation:
// the SDK runs in this test process, and in Go an unrecovered panic on any
// goroutine aborts the whole `go test` binary, so a crash fails the run
// before any expectation can be evaluated as passing.
func (p *chaosProbe) processStillAlive() bool {
	return true
}

// ---- failover + ordering observability (read the public accessors added for
// this epic; nil-safe before the client is constructed) ----

func (p *chaosProbe) ready() bool {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	if c == nil {
		return false
	}
	return c.Ready()
}

func (p *chaosProbe) resolvedFrom() string {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	if c == nil {
		return ""
	}
	return c.ResolvedFrom()
}

func (p *chaosProbe) heldGeneration() int {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	if c == nil {
		return 0
	}
	return c.HeldGeneration()
}

func (p *chaosProbe) configInstallCount() int {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	if c == nil {
		return 0
	}
	return c.ConfigInstallCount()
}

func (p *chaosProbe) sseFailedOverToSecondary() bool {
	p.mu.Lock()
	c := p.client
	p.mu.Unlock()
	if c == nil {
		return false
	}
	return c.SSEFailedOverToSecondary()
}

func (p *chaosProbe) sdkLogMatches(level string, re *regexp.Regexp) int {
	p.logMu.Lock()
	defer p.logMu.Unlock()
	out := 0
	for _, line := range strings.Split(p.logBuf.String(), "\n") {
		if line == "" {
			continue
		}
		// chaosLogHandler writes every line as `level=<SLOG LEVEL> msg=...`;
		// sdkLog counts lines at exactly that level.
		if level != "" && !strings.HasPrefix(line, "level="+slogLevelName(level)+" ") {
			continue
		}
		if re == nil || re.MatchString(line) {
			out++
		}
	}
	return out
}

// slogLevelName maps a scenario log level ('warn', 'error', ...) to the name
// slog prints for it.
func slogLevelName(level string) string {
	switch strings.ToLower(level) {
	case "warn", "warning":
		return "WARN"
	}
	return strings.ToUpper(level)
}

// slogHandler collects everything the SDK logs into logBuf so sdkLog can scan.
type chaosLogHandler struct {
	p *chaosProbe
}

func (h *chaosLogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *chaosLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.p.logMu.Lock()
	fmt.Fprintf(&h.p.logBuf, "level=%s msg=%q", r.Level.String(), r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&h.p.logBuf, " %s=%v", a.Key, a.Value)
		return true
	})
	h.p.logBuf.WriteByte('\n')
	h.p.logMu.Unlock()
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
}

// exprVerdict is the tri-state outcome of evaluating an assert expression.
type exprVerdict int

const (
	verdictFail exprVerdict = iota
	verdictPass
	// verdictSkipped: this rig cannot evaluate the expression, for a stated
	// reason. A standalone skipped expectation is reported as SKIPPED, never
	// as PASS. Inside AND/OR a skipped leaf is neutral: it neither satisfies
	// nor fails the compound, and the other leaves are still enforced.
	verdictSkipped
)

func (v exprVerdict) String() string {
	switch v {
	case verdictPass:
		return "pass"
	case verdictSkipped:
		return "skipped"
	}
	return "fail"
}

// skipNote records one leaf the rig skipped and why.
type skipNote struct {
	expr   string
	reason string
}

func (n skipNote) String() string { return n.expr + " — SKIPPED: " + n.reason }

// exprResult is what evaluate returns: the verdict, a reason string for
// FAIL/SKIP logging, and every skipped leaf encountered (also on a PASS of a
// compound expression, so the report can show which leaves were not checked).
type exprResult struct {
	verdict exprVerdict
	why     string
	skipped []skipNote
}

// serverMetricSkipReason is why server_metric(...) is skipped rather than
// evaluated (qfg-goi1.1.1). It used to be stubbed to 0, which made every
// `server_metric(...) == 0` expectation pass without checking anything.
const serverMetricSkipReason = "server-side metric; api-delivery exports metrics via OTLP push only, " +
	"no scrape endpoint in the rig; server lag is covered by the staging drill qfg-47c2.19 " +
	"and the QuonfigSubscriberLagHigh alert"

// evaluate parses and evaluates one assert string against the current state.
func evaluate(expr string, ec *evalCtx) exprResult {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return exprResult{verdict: verdictPass}
	}
	// Top-level: split on " OR " (lowest precedence). Each OR-leaf is then
	// split on " AND " (higher precedence). Parentheses are not used by any
	// scenario shipped to date.
	if strings.Contains(expr, " OR ") {
		parts := splitOutsideQuotesAndRegex(expr, " OR ")
		var reasons []string
		var skipped []skipNote
		anyFail := false
		for _, p := range parts {
			r := evaluate(p, ec)
			skipped = append(skipped, r.skipped...)
			switch r.verdict {
			case verdictPass:
				return exprResult{verdict: verdictPass, skipped: skipped}
			case verdictFail:
				anyFail = true
				reasons = append(reasons, r.why)
			}
		}
		if anyFail {
			return exprResult{verdict: verdictFail, why: "OR: " + strings.Join(reasons, " | "), skipped: skipped}
		}
		return exprResult{verdict: verdictSkipped, why: joinSkipNotes(skipped), skipped: skipped}
	}
	if strings.Contains(expr, " AND ") {
		parts := splitOutsideQuotesAndRegex(expr, " AND ")
		var skipped []skipNote
		anyPass := false
		for _, p := range parts {
			r := evaluate(p, ec)
			skipped = append(skipped, r.skipped...)
			switch r.verdict {
			case verdictFail:
				return exprResult{verdict: verdictFail, why: "AND: " + r.why, skipped: skipped}
			case verdictPass:
				anyPass = true
			}
		}
		if anyPass {
			return exprResult{verdict: verdictPass, skipped: skipped}
		}
		return exprResult{verdict: verdictSkipped, why: joinSkipNotes(skipped), skipped: skipped}
	}
	return evalLeaf(expr, ec)
}

func joinSkipNotes(notes []skipNote) string {
	parts := make([]string, len(notes))
	for i, n := range notes {
		parts[i] = n.String()
	}
	return strings.Join(parts, " | ")
}

// chaosSkipTally collects every skipped leaf across one chaos run so the run
// ends with a single "skipped expressions" summary (qfg-goi1.1.1).
type chaosSkipTally struct {
	mu      sync.Mutex
	order   []string
	counts  map[string]int
	reasons map[string]string
}

func newChaosSkipTally() *chaosSkipTally {
	return &chaosSkipTally{counts: map[string]int{}, reasons: map[string]string{}}
}

func (t *chaosSkipTally) record(notes []skipNote) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, n := range notes {
		if _, seen := t.counts[n.expr]; !seen {
			t.order = append(t.order, n.expr)
			t.reasons[n.expr] = n.reason
		}
		t.counts[n.expr]++
	}
}

// report logs the run-end tally. It always prints, so "no skips" is visible
// too.
func (t *chaosSkipTally) report(tb testing.TB) {
	tb.Helper()
	t.mu.Lock()
	defer t.mu.Unlock()
	total := 0
	for _, c := range t.counts {
		total += c
	}
	tb.Logf("skipped expressions: %d occurrence(s) of %d distinct expression(s)", total, len(t.order))
	for _, e := range t.order {
		tb.Logf("  SKIPPED x%d  %s — %s", t.counts[e], e, t.reasons[e])
	}
}

// reportChaosExpectation logs one expectation's final outcome and records its
// skipped leaves in the tally. A FAIL calls t.Errorf so red shows up in
// `go test` output; a SKIP is logged with its reason and never counts as PASS.
func reportChaosExpectation(t *testing.T, tally *chaosSkipTally, idx int, exp chaosExpectation, verdict exprVerdict, hitAt time.Duration, lastReason string, notes []skipNote) {
	t.Helper()
	tally.record(notes)
	switch verdict {
	case verdictPass:
		suffix := ""
		if len(notes) > 0 {
			suffix = "  [skipped leaf: " + joinSkipNotes(notes) + "]"
		}
		t.Logf("PASS  exp[%d] within=%dms hold=%dms: %s  (hit at %s)%s", idx, exp.WithinMs, exp.MustHoldForMs, exp.Assert, hitAt, suffix)
	case verdictSkipped:
		t.Logf("SKIP  exp[%d] within=%dms hold=%dms: %s  — %s", idx, exp.WithinMs, exp.MustHoldForMs, exp.Assert, lastReason)
	default:
		t.Errorf("FAIL  exp[%d] within=%dms hold=%dms: %s  — last reason: %s", idx, exp.WithinMs, exp.MustHoldForMs, exp.Assert, lastReason)
	}
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
	reSDKMetric   = regexp.MustCompile(`^client\.sdkMetric\(\s*'([^']+)'\s*(?:,\s*(layer|callback)=\s*'([^']+)'\s*)?\)\s*(>=|<=|==|!=|<|>)\s*(\d+)$`)
	reServerMet   = regexp.MustCompile(`^server_metric\(\s*'([^']+)'\s*\)\s*(>=|<=|==|!=|<|>)\s*(\d+)$`)
	reSDKLog      = regexp.MustCompile(`^client\.sdkLog\(\s*'([^']+)'\s*,\s*/(.+)/(i?)\s*\)\s*(>=|<=|==|!=|<|>)\s*(\d+)$`)

	// Init-failure vocabulary (00-boot-both-down-then-restore).
	reInitState   = regexp.MustCompile(`^client\.initState\(\)\s*(==|!=)\s*'([^']*)'$`)
	reGetString   = regexp.MustCompile(`^client\.getString\(\s*'([^']*)'\s*,\s*'([^']*)'\s*\)\s*(==|!=)\s*'([^']*)'$`)
	reReadyEvents = regexp.MustCompile(`^client\.readyEventCount\(\)\s*(>=|<=|==|!=|<|>)\s*(\d+)$`)

	// Failover + ordering vocabulary.
	reReady        = regexp.MustCompile(`^client\.ready\(\)\s*==\s*(true|false)$`)
	reResolvedFrom = regexp.MustCompile(`^client\.resolvedFrom\(\)\s*(==|!=)\s*'([^']+)'$`)
	reHeldGen      = regexp.MustCompile(`^client\.heldGeneration\(\)\s*(>=|<=|==|!=|<|>)\s*(-?\d+)$`)
	reInstallCnt   = regexp.MustCompile(`^client\.configInstallCount\(\)\s*(>=|<=|==|!=|<|>)\s*(-?\d+)$`)
	reSSEFailover  = regexp.MustCompile(`^client\.sseFailedOverToSecondary\(\)\s*==\s*(true|false)$`)
)

// leafResult turns a leaf's (ok, why) into an exprResult.
func leafResult(ok bool, why string) exprResult {
	if ok {
		return exprResult{verdict: verdictPass, why: why}
	}
	return exprResult{verdict: verdictFail, why: why}
}

func evalLeaf(expr string, ec *evalCtx) exprResult {
	expr = strings.TrimSpace(expr)
	if m := reConnStateEq.FindStringSubmatch(expr); m != nil {
		got := ec.probe.connectionState()
		want := m[2]
		switch m[1] {
		case "==":
			ok := got == want
			return leafResult(ok, fmt.Sprintf("connectionState=%s want %s", got, want))
		case "!=":
			ok := got != want
			return leafResult(ok, fmt.Sprintf("connectionState=%s want != %s", got, want))
		}
	}
	if m := reReady.FindStringSubmatch(expr); m != nil {
		want := m[1] == "true"
		got := ec.probe.ready()
		return leafResult(got == want, fmt.Sprintf("ready=%v want %v", got, want))
	}
	if m := reResolvedFrom.FindStringSubmatch(expr); m != nil {
		got := ec.probe.resolvedFrom()
		want := m[2]
		switch m[1] {
		case "==":
			return leafResult(got == want, fmt.Sprintf("resolvedFrom=%q want %q", got, want))
		case "!=":
			return leafResult(got != want, fmt.Sprintf("resolvedFrom=%q want != %q", got, want))
		}
	}
	if m := reHeldGen.FindStringSubmatch(expr); m != nil {
		got := int64(ec.probe.heldGeneration())
		want, _ := strconv.ParseInt(m[2], 10, 64)
		ok := compareInt(m[1], got, want)
		return leafResult(ok, fmt.Sprintf("heldGeneration=%d %s %d", got, m[1], want))
	}
	if m := reInstallCnt.FindStringSubmatch(expr); m != nil {
		got := int64(ec.probe.configInstallCount())
		want, _ := strconv.ParseInt(m[2], 10, 64)
		ok := compareInt(m[1], got, want)
		return leafResult(ok, fmt.Sprintf("configInstallCount=%d %s %d", got, m[1], want))
	}
	if m := reSSEFailover.FindStringSubmatch(expr); m != nil {
		want := m[1] == "true"
		got := ec.probe.sseFailedOverToSecondary()
		return leafResult(got == want, fmt.Sprintf("sseFailedOverToSecondary=%v want %v", got, want))
	}
	if m := reFallbackEq.FindStringSubmatch(expr); m != nil {
		want := m[1] == "true"
		got := ec.probe.fallbackPollerActive()
		return leafResult(got == want, fmt.Sprintf("fallbackPollerActive=%v want %v", got, want))
	}
	if m := reProcAliveEq.FindStringSubmatch(expr); m != nil {
		want := m[1] == "true"
		got := ec.probe.processStillAlive()
		return leafResult(got == want, fmt.Sprintf("processStillAlive=%v want %v", got, want))
	}
	if m := reLastRefresh.FindStringSubmatch(expr); m != nil {
		ago, _ := strconv.Atoi(m[2])
		last := ec.probe.lastSuccessfulRefreshMs()
		threshold := time.Now().UnixMilli() - int64(ago)
		ok := compareInt(m[1], last, threshold)
		return leafResult(ok, fmt.Sprintf("lastSuccessfulRefresh=%d %s (now()-%d)=%d", last, m[1], ago, threshold))
	}
	if m := reSDKMetric.FindStringSubmatch(expr); m != nil {
		labels := map[string]string{}
		if m[2] != "" {
			labels[m[2]] = m[3]
		}
		got, known := ec.probe.sdkMetric(m[1], labels)
		if !known {
			return exprResult{verdict: verdictFail, why: fmt.Sprintf("unknown sdkMetric %q: the chaos probe does not implement it", m[1])}
		}
		want, _ := strconv.ParseFloat(m[5], 64)
		ok := compareFloat(m[4], got, want)
		return leafResult(ok, fmt.Sprintf("sdkMetric(%s,%s=%s)=%v %s %v", m[1], m[2], m[3], got, m[4], want))
	}
	if m := reServerMet.FindStringSubmatch(expr); m != nil {
		note := skipNote{expr: expr, reason: serverMetricSkipReason}
		return exprResult{verdict: verdictSkipped, why: note.String(), skipped: []skipNote{note}}
	}
	if m := reSDKLog.FindStringSubmatch(expr); m != nil {
		level, pattern, flags := m[1], m[2], m[3]
		src := pattern
		if flags == "i" {
			src = "(?i)" + pattern
		}
		re, err := regexp.Compile(src)
		if err != nil {
			return leafResult(false, fmt.Sprintf("bad regex %q: %v", pattern, err))
		}
		n := ec.probe.sdkLogMatches(level, re)
		want, _ := strconv.Atoi(m[5])
		ok := compareInt(m[4], int64(n), int64(want))
		return leafResult(ok, fmt.Sprintf("sdkLog(%s,/%s/%s)=%d %s %d", level, pattern, flags, n, m[4], want))
	}
	if m := reInitState.FindStringSubmatch(expr); m != nil {
		got := ec.probe.initState()
		ok := (got == m[2]) == (m[1] == "==")
		return leafResult(ok, fmt.Sprintf("initState=%q %s %q", got, m[1], m[2]))
	}
	if m := reGetString.FindStringSubmatch(expr); m != nil {
		got := ec.probe.getString(m[1], m[2])
		ok := (got == m[4]) == (m[3] == "==")
		return leafResult(ok, fmt.Sprintf("getString(%s)=%q %s %q", m[1], got, m[3], m[4]))
	}
	if m := reReadyEvents.FindStringSubmatch(expr); m != nil {
		got := ec.probe.readyEventCount()
		want, _ := strconv.ParseInt(m[2], 10, 64)
		ok := compareInt(m[1], got, want)
		return leafResult(ok, fmt.Sprintf("readyEventCount=%d %s %d", got, m[1], want))
	}
	return leafResult(false, fmt.Sprintf("unrecognized expression: %s", expr))
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
