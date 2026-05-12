//go:build chaos

// Cross-SDK chaos harness — sdk-go runner (bead qfg-47c2.4).
//
// Wires sdk-go's test runner to integration-test-data/chaos/. The shared
// launcher (../integration-test-data/chaos/start-chaos.sh) must already have
// booted toxiproxy; this test reconfigures the seeded SSE/HTTP proxies to
// point at a locally-spawned api-delivery (FIXTURE_DIR mode) and runs each
// scenario against the SDK.
//
// Run with: scripts/run-chaos.sh   (or `go test -tags chaos -run TestChaos`
// after starting the harness + api-delivery manually).
//
// Environment knobs:
//   TOXIPROXY_URL            admin API base       (default http://127.0.0.1:8474)
//   CHAOS_SSE_PORT           host SSE port        (default 18550)
//   CHAOS_HTTP_PORT          host HTTP port       (default 18551)
//   CHAOS_API_DELIVERY_PORT  port to bind the spawned api-delivery on (default 6550)
//   CHAOS_API_DELIVERY_URL   if set, use an externally-running api-delivery
//                            instead of spawning one
//   CHAOS_UPSTREAM_HOST      hostname toxiproxy uses to reach api-delivery
//                            (default host.docker.internal)
//   CHAOS_ONLY               comma list of scenario numbers to run, e.g. "02,05,07,09"
//   CHAOS_SKIP               comma list of scenario numbers to skip
//   CHAOS_POLL_MS            expectation poll interval (default 250)

package quonfig

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("chaos tests skipped in -short mode")
	}

	toxiURL := envOrDefault("TOXIPROXY_URL", "http://127.0.0.1:8474")
	tp := newToxiproxy(toxiURL)
	if err := tp.ping(context.Background()); err != nil {
		t.Skipf("toxiproxy not reachable at %s: %v — run integration-test-data/chaos/start-chaos.sh first", toxiURL, err)
	}

	apiURL := os.Getenv("CHAOS_API_DELIVERY_URL")
	if apiURL == "" {
		port, _ := strconv.Atoi(envOrDefault("CHAOS_API_DELIVERY_PORT", "6550"))
		apiURL = startChaosAPIDelivery(t, port)
	}

	// Reconfigure the seeded proxies to forward to our api-delivery. Toxiproxy
	// runs in a docker container; reach the host via host.docker.internal.
	upstreamHost := envOrDefault("CHAOS_UPSTREAM_HOST", "host.docker.internal")
	upstreamPort := parsePortFromURL(t, apiURL)
	tp.upsertProxy(t, "sse", "0.0.0.0:18550", fmt.Sprintf("%s:%d", upstreamHost, upstreamPort))
	tp.upsertProxy(t, "http", "0.0.0.0:18551", fmt.Sprintf("%s:%d", upstreamHost, upstreamPort))

	ssePort, _ := strconv.Atoi(envOrDefault("CHAOS_SSE_PORT", "18550"))
	httpPort, _ := strconv.Atoi(envOrDefault("CHAOS_HTTP_PORT", "18551"))

	files, err := filepath.Glob(filepath.Join(chaosScenariosDir(), "*.yaml"))
	if err != nil {
		t.Fatalf("glob scenarios: %v", err)
	}
	sort.Strings(files)
	only := splitCSV(os.Getenv("CHAOS_ONLY"))
	skip := splitCSV(os.Getenv("CHAOS_SKIP"))
	pollMs, _ := strconv.Atoi(envOrDefault("CHAOS_POLL_MS", "250"))
	pollInterval := time.Duration(pollMs) * time.Millisecond
	if pollInterval <= 0 {
		pollInterval = 250 * time.Millisecond
	}

	for _, file := range files {
		base := filepath.Base(file)
		num := scenarioNumber(base)
		if len(only) > 0 && !containsStr(only, num) {
			continue
		}
		if containsStr(skip, num) {
			continue
		}
		t.Run(strings.TrimSuffix(base, ".yaml"), func(t *testing.T) {
			scenario := loadChaosScenario(t, file)
			for _, run := range scenario.Tests {
				t.Run(safeRunName(run.Name), func(t *testing.T) {
					runChaosScenario(t, tp, run, httpPort, ssePort, pollInterval)
				})
			}
		})
	}
}

// runChaosScenario executes one scenario end-to-end against a fresh SDK
// client and records pass/fail per expectation. Failed expectations call
// t.Errorf so red shows up in test output — that is the whole point of this
// bead.
func runChaosScenario(t *testing.T, tp *toxiproxyClient, run chaosScenarioRun, httpPort, ssePort int, poll time.Duration) {
	t.Helper()
	// Reset proxy state (idempotent — leftover toxics from a prior scenario
	// would make this one start dirty).
	tp.clearToxics(t, "sse")
	tp.clearToxics(t, "http")
	tp.setEnabled(t, "sse", true)
	tp.setEnabled(t, "http", true)

	probe := newChaosProbe()
	logger := slog.New(&chaosLogHandler{p: probe})

	apiURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	streamURL := fmt.Sprintf("http://127.0.0.1:%d", ssePort)

	opts := []Option{
		WithAPIKey("test-backend-key"),
		WithAPIURLs([]string{apiURL}),
		withTestStreamURLOverride(streamURL),
		WithSSE(true),
		WithSSEStateCallback(func(connected bool) { probe.onSSEState(connected) }),
		WithOnConfigUpdate(func() { probe.onConfigUpdate() }),
		WithLogger(logger),
		WithAllTelemetryDisabled(),
		WithInitTimeout(15 * time.Second),
		WithOnInitFailure(ReturnZeroValue),
		// Compress the production 90s SSE read deadline to 5s for chaos
		// scenarios — the spec is "3x the server heartbeat", and the
		// harness uses a 30s heartbeat in production but we don't have
		// minutes per scenario in CI. 5s gives the same mechanism
		// (deadline trips → drop → reconnect) at test cadence so e.g.
		// scenario 07's within_ms=15000 is reachable.
		withTestSSEReadTimeout(5 * time.Second),
		// Enable Layer 2 fallback polling per qfg-47c2.20. Scenarios 5/6
		// expect the poller to engage after the 120s default threshold;
		// other scenarios still benefit from the poller being available
		// as a safety net.
		WithFallbackPoll(true, 30*time.Second),
	}

	// Scenario 10 (user_callback: throw) — a user callback panic that the
	// SDK's invokeOnEnvelope recover() boundary must catch. The panic
	// propagates installEnvelope → sseClient.OnEnvelope wrapper →
	// invokeOnEnvelope, which logs at ERROR ("OnEnvelope callback panicked")
	// and bumps quonfig_sdk_worker_restart_total{reason="callback_panic"}.
	// Do NOT wrap the user callback in its own defer/recover — that would
	// swallow the panic before the SDK's recovery path could fire, defeating
	// the scenario (qfg-47c2.30).
	if run.Setup.UserCallback == "throw" {
		opts = append(opts, WithOnConfigUpdate(func() {
			probe.onConfigUpdate()
			panic("simulated user-callback panic for chaos scenario 10")
		}))
	}

	client, err := NewClient(opts...)
	if err != nil {
		t.Logf("client init failed: %v — continuing (the scenario may still observe disconnected state)", err)
	} else {
		t.Cleanup(client.Close)
		probe.setClient(client)
	}

	// Schedule chaos events.
	baseline := time.Now()
	injections := make(map[string]*injectionState)
	for _, ev := range run.Chaos {
		ev := ev // capture
		when := baseline.Add(time.Duration(ev.AtMs) * time.Millisecond)
		go func() {
			delay := time.Until(when)
			if delay > 0 {
				time.Sleep(delay)
			}
			switch {
			case ev.Inject != nil:
				st := applyInject(t, tp, ev.Inject)
				if ev.Inject.Name != "" && st != nil {
					injections[ev.Inject.Name] = st
				}
				t.Logf("[%6dms] inject %+v", ev.AtMs, ev.Inject)
			case ev.Clear != "":
				clearInject(t, tp, injections[ev.Clear])
				delete(injections, ev.Clear)
				t.Logf("[%6dms] clear %s", ev.AtMs, ev.Clear)
			case ev.Process != nil:
				applyProcess(t, tp, ev.Process, baseline)
				t.Logf("[%6dms] process %+v", ev.AtMs, ev.Process)
			}
		}()
	}

	wallClock := time.Duration(run.Setup.WallClockSeconds) * time.Second
	if wallClock <= 0 {
		wallClock = 30 * time.Second
	}

	ec := &evalCtx{
		probe:        probe,
		serverMetric: func(name string) float64 { return 0 },
	}

	type expState struct {
		idx        int
		exp        chaosExpectation
		hitAt      time.Duration
		heldSince  time.Time
		passed     bool
		failed     bool // within_ms elapsed without pass
		lastReason string
	}
	states := make([]*expState, len(run.Expectations))
	for i, exp := range run.Expectations {
		states[i] = &expState{idx: i, exp: exp}
	}

	deadline := baseline.Add(wallClock)
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for now := time.Now(); now.Before(deadline); now = <-ticker.C {
		elapsed := now.Sub(baseline)
		for _, s := range states {
			if s.passed || s.failed {
				continue
			}
			ok, why := evaluate(s.exp.Assert, ec)
			s.lastReason = why
			if ok {
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
			// Definitive fail: within_ms has elapsed and we either never hit,
			// or we hit but never held long enough. Hold-for failures still
			// have a chance until within_ms; after that it's terminal.
			if !s.passed && elapsed > time.Duration(s.exp.WithinMs)*time.Millisecond {
				s.failed = true
			}
		}
		allTerminal := true
		for _, s := range states {
			if !s.passed && !s.failed {
				allTerminal = false
				break
			}
		}
		if allTerminal {
			break
		}
	}

	// Anything still not passed at this point is a fail.
	for _, s := range states {
		if !s.passed {
			s.failed = true
		}
	}

	// Report — failed expectations fail the test so the red proof shows up
	// in `go test` output.
	passCount, failCount := 0, 0
	for _, s := range states {
		if s.passed {
			passCount++
			t.Logf("PASS  exp[%d] within=%dms hold=%dms: %s  (hit at %s)", s.idx, s.exp.WithinMs, s.exp.MustHoldForMs, s.exp.Assert, s.hitAt)
		} else {
			failCount++
			t.Errorf("FAIL  exp[%d] within=%dms hold=%dms: %s  — last reason: %s", s.idx, s.exp.WithinMs, s.exp.MustHoldForMs, s.exp.Assert, s.lastReason)
		}
	}
	t.Logf("scenario summary: %d passed, %d failed (state=%s, sdkMetric.layer1=%v, fallback=%v, lastRefreshMs=%v)",
		passCount, failCount,
		probe.connectionState(),
		probe.sdkMetric("quonfig_sdk_worker_restart_total", map[string]string{"layer": "1"}),
		probe.fallbackPollerActive(),
		probe.lastSuccessfulRefreshMs(),
	)
}

// startChaosAPIDelivery builds and starts api-delivery on the given port in
// fixture mode. Mirrors integration_test.go's startTestServer but binds a
// caller-specified port so toxiproxy's seeded upstream can target it via
// host.docker.internal.
func startChaosAPIDelivery(t *testing.T, port int) string {
	t.Helper()
	fixtureDir := chaosIntegrationDataDir()
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Fatalf("integration-test-data fixtures not found at %s: %v", fixtureDir, err)
	}
	keysPath := chaosFixtureKeysPath()
	if _, err := os.Stat(keysPath); err != nil {
		t.Fatalf("fixture SDK keys not found at %s: %v", keysPath, err)
	}

	serverDir := filepath.Join(chaosProjectRoot(), "api-delivery")
	binary := filepath.Join(t.TempDir(), "api-delivery")
	build := exec.Command("go", "build", "-o", binary, "./cmd/server")
	build.Dir = serverDir
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build api-delivery: %v\n%s", err, out)
	}

	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"FIXTURE_DIR="+fixtureDir,
		"SDK_KEYS_FILE="+keysPath,
		"QUONFIG_ENVIRONMENT=development",
		// Compress the SSE keepalive cadence so it fits inside the chaos
		// read deadline (testSSEReadTimeout=5s above). Production runs at
		// 30s; without this override the deadline trips before the first
		// keepalive arrives, making scenario 01 baseline unreachable
		// (qfg-47c2.28).
		"SSE_HEARTBEAT_INTERVAL=1s",
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start api-delivery: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if dialOK("127.0.0.1", port) {
			time.Sleep(100 * time.Millisecond)
			return base
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("api-delivery did not start on :%d within 15s", port)
	return ""
}

// ----- tiny helpers -----

func envOrDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func containsStr(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func scenarioNumber(filename string) string {
	if i := strings.Index(filename, "-"); i > 0 {
		return filename[:i]
	}
	return filename
}

func safeRunName(s string) string {
	r := strings.NewReplacer(" ", "_", "/", "_", "(", "", ")", "")
	return r.Replace(s)
}

func parsePortFromURL(t *testing.T, u string) int {
	t.Helper()
	i := strings.LastIndex(u, ":")
	if i < 0 {
		t.Fatalf("no port in URL %q", u)
	}
	rest := u[i+1:]
	if j := strings.IndexAny(rest, "/?"); j >= 0 {
		rest = rest[:j]
	}
	port, err := strconv.Atoi(rest)
	if err != nil {
		t.Fatalf("parse port from %q: %v", u, err)
	}
	return port
}

func dialOK(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
