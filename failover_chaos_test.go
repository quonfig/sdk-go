//go:build chaos

// Failover + canonical-ordering chaos runners for sdk-go (bead qfg-7h5d.1.3).
//
// These two runners consume the shared corpus rigs added in qfg-7h5d.1.2:
//
//   scenarios-failover/  (f01-f05) — ONE fixture upstream behind TWO proxies
//     (primary 'http' leg + 'secondary' leg). Faults hit the primary leg only;
//     the SDK must fail the HTTP config fetch over to the secondary and keep
//     serving, fast (well inside InitTimeout). SSE is asserted to NOT repoint.
//
//   scenarios-ordering/  (o01-o04) — TWO fixture upstreams pinned to divergent
//     Meta.generations. The SDK must end up holding the higher generation and
//     an established client must never regress to an older one.
//
// Only toxiproxy needs to be running (start it with
// ../integration-test-data/chaos/start-chaos.sh — no --with-upstream needed).
// Each runner spawns its own api-delivery fixture upstream(s) and repoints the
// seeded 'http'/'secondary'/'sse' proxies at them, exactly like TestChaos does
// for the single-upstream rig. Spawning the upstreams here (rather than via the
// launcher) lets the ordering runner pin a different generation per scenario.
//
// Run with: scripts/run-failover-chaos.sh   (or, with toxiproxy already up,
// `go test -tags chaos -run 'TestFailoverChaos|TestOrderingChaos'`).
//
// RED baseline captured by this bead (see the plan, Phase 1):
//   - f02 (primary hang) FAILS — no per-URL config-fetch timeout yet, so a hung
//     primary starves the secondary until InitTimeout. Turns green in qfg-7h5d.1.4.
//   - o02 (secondary older) FAILS — installEnvelope installs unconditionally, so
//     a failover fetch of the older secondary regresses the held generation.
//     Turns green in qfg-7h5d.1.5 (reject-older install guard).

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
	"strings"
	"testing"
	"time"
)

// Host ports the launcher maps the seeded proxies to (docker-compose.yml).
const (
	rigPrimaryPort   = 18551 // 'http' proxy — primary HTTP leg
	rigSecondaryPort = 18552 // 'secondary' proxy — secondary HTTP leg
	rigSSEPort       = 18550 // 'sse' proxy — live stream (primary leg only)
	rigInitTimeout   = 8 * time.Second
)

func chaosFailoverScenariosDir() string {
	return filepath.Join(chaosProjectRoot(), "integration-test-data", "chaos", "scenarios-failover")
}

func chaosOrderingScenariosDir() string {
	return filepath.Join(chaosProjectRoot(), "integration-test-data", "chaos", "scenarios-ordering")
}

// TestFailoverChaos drives scenarios-failover/ against one fixture upstream sat
// behind the primary + secondary proxies. Faults are injected on the primary
// leg only; the SDK must resolve off the secondary leg, fast.
func TestFailoverChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("failover chaos tests skipped in -short mode")
	}
	tp := dialRigToxiproxy(t)
	binary := buildChaosAPIDelivery(t)

	// One upstream; both HTTP legs and the SSE leg point at it (identical content
	// proves failover routing, not divergent data — that's the ordering rig).
	upstreamHost := envOrDefault("CHAOS_UPSTREAM_HOST", "host.docker.internal")
	port := freePort(t)
	spawnChaosUpstream(t, binary, port, 0)
	reconfigureRigProxies(t, tp, upstreamHost, port, port)

	files := globScenarios(t, chaosFailoverScenariosDir())
	for _, file := range files {
		base := strings.TrimSuffix(filepath.Base(file), ".yaml")
		scenario := loadChaosScenario(t, file)
		t.Run(base, func(t *testing.T) {
			for _, run := range scenario.Tests {
				t.Run(safeRunName(run.Name), func(t *testing.T) {
					runRigScenario(t, tp, run, false)
				})
			}
		})
	}
}

// TestOrderingChaos drives scenarios-ordering/ against TWO fixture upstreams
// pinned to the generations declared in each scenario's setup.upstreams. A
// background Refresh loop models ongoing config polling so the canonical-ordering
// guard (and its absence) is exercised on the failover/refresh install path.
func TestOrderingChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("ordering chaos tests skipped in -short mode")
	}
	tp := dialRigToxiproxy(t)
	binary := buildChaosAPIDelivery(t)
	upstreamHost := envOrDefault("CHAOS_UPSTREAM_HOST", "host.docker.internal")

	files := globScenarios(t, chaosOrderingScenariosDir())
	for _, file := range files {
		base := strings.TrimSuffix(filepath.Base(file), ".yaml")
		scenario := loadChaosScenario(t, file)
		t.Run(base, func(t *testing.T) {
			for _, run := range scenario.Tests {
				t.Run(safeRunName(run.Name), func(t *testing.T) {
					primaryGen, secondaryGen := upstreamGenerations(t, run.Setup.Upstreams)
					// Distinct ports per scenario so the prior scenario's
					// upstreams (torn down on its t.Cleanup) can't collide.
					primaryUpstream := freePort(t)
					secondaryUpstream := freePort(t)
					spawnChaosUpstream(t, binary, primaryUpstream, primaryGen)
					spawnChaosUpstream(t, binary, secondaryUpstream, secondaryGen)
					reconfigureRigProxies(t, tp, upstreamHost, primaryUpstream, secondaryUpstream)
					runRigScenario(t, tp, run, true)
				})
			}
		})
	}
}

// runRigScenario stands up a fresh SDK client pointed at [primary, secondary],
// schedules the scenario's chaos events against the primary leg, optionally
// drives a Refresh loop, then evaluates every expectation on a poll timer.
func runRigScenario(t *testing.T, tp *toxiproxyClient, run chaosScenarioRun, driveRefresh bool) {
	t.Helper()
	// Start from a clean proxy state — no leftover toxics, all legs enabled.
	for _, p := range []string{"http", "secondary", "sse"} {
		tp.clearToxics(t, p)
		tp.setEnabled(t, p, true)
	}

	probe := newChaosProbe()
	logger := slog.New(&chaosLogHandler{p: probe})

	primaryURL := fmt.Sprintf("http://127.0.0.1:%d", rigPrimaryPort)
	secondaryURL := fmt.Sprintf("http://127.0.0.1:%d", rigSecondaryPort)
	streamURL := fmt.Sprintf("http://127.0.0.1:%d", rigSSEPort)

	sseEnabled := run.Setup.SSEEndpoint != "" && run.Setup.SSEEndpoint != "disabled"
	opts := []Option{
		WithSdkKey("test-backend-key"),
		WithAPIURLs([]string{primaryURL, secondaryURL}),
		WithAllTelemetryDisabled(),
		WithInitTimeout(rigInitTimeout),
		WithOnInitFailure(ReturnZeroValue),
		WithLogger(logger),
		WithSSE(sseEnabled),
		// Background polling is driven explicitly via the Refresh loop below for
		// the ordering rig; leave the fallback poller off so install accounting
		// (o04's configInstallCount) is deterministic.
		WithFallbackPoll(false, 0),
	}
	if sseEnabled {
		opts = append(opts,
			withTestStreamURLOverride(streamURL),
			withTestSSEReadTimeout(5*time.Second),
			WithSSEStateCallback(func(connected bool) { probe.onSSEState(connected) }),
		)
	}

	client, err := NewClient(opts...)
	if err != nil {
		t.Logf("client init returned: %v — continuing (the scenario may still observe the failure)", err)
	} else {
		t.Cleanup(client.Close)
		probe.setClient(client)
	}

	baseline := time.Now()

	// Schedule chaos events. The failover-rig aliases are self-restoring (they
	// carry their own duration), so there are no `clear` events to track.
	for _, ev := range run.Chaos {
		ev := ev
		if ev.Inject == nil {
			continue
		}
		when := baseline.Add(time.Duration(ev.AtMs) * time.Millisecond)
		go func() {
			if d := time.Until(when); d > 0 {
				time.Sleep(d)
			}
			applyFailoverInject(t, tp, ev.Inject)
			t.Logf("[%6dms] inject %+v", ev.AtMs, ev.Inject)
		}()
	}

	// Ordering rig: model ongoing config polling. Each Refresh re-runs the
	// [primary, secondary] failover fetch; without the reject-older guard a
	// failover to an older secondary regresses the held generation (o02 red).
	if driveRefresh && client != nil {
		stop := make(chan struct{})
		t.Cleanup(func() { close(stop) })
		go func() {
			ticker := time.NewTicker(750 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					_ = client.Refresh()
				}
			}
		}()
	}

	evalRigExpectations(t, run, baseline, probe)
}

// evalRigExpectations polls each expectation until it passes (and holds for
// must_hold_for_ms) or its within_ms deadline elapses. Failures call t.Errorf
// so red shows up in `go test` output. Mirrors the eval loop in runChaosScenario.
func evalRigExpectations(t *testing.T, run chaosScenarioRun, baseline time.Time, probe *chaosProbe) {
	t.Helper()
	ec := &evalCtx{probe: probe, serverMetric: func(string) float64 { return 0 }}

	wallClock := time.Duration(run.Setup.WallClockSeconds) * time.Second
	if wallClock <= 0 {
		wallClock = 30 * time.Second
	}

	type expState struct {
		exp        chaosExpectation
		heldSince  time.Time
		hitAt      time.Duration
		passed     bool
		failed     bool
		lastReason string
	}
	states := make([]*expState, len(run.Expectations))
	for i, exp := range run.Expectations {
		states[i] = &expState{exp: exp}
	}

	deadline := baseline.Add(wallClock)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for now := time.Now(); now.Before(deadline); now = <-ticker.C {
		elapsed := now.Sub(baseline)
		allTerminal := true
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
			if !s.passed && elapsed > time.Duration(s.exp.WithinMs)*time.Millisecond {
				s.failed = true
			}
			if !s.passed && !s.failed {
				allTerminal = false
			}
		}
		if allTerminal {
			break
		}
	}

	for i, s := range states {
		if !s.passed {
			s.failed = true
		}
		if s.passed {
			t.Logf("PASS  exp[%d] within=%dms hold=%dms: %s  (hit at %s)", i, s.exp.WithinMs, s.exp.MustHoldForMs, s.exp.Assert, s.hitAt)
		} else {
			t.Errorf("FAIL  exp[%d] within=%dms hold=%dms: %s  — last reason: %s", i, s.exp.WithinMs, s.exp.MustHoldForMs, s.exp.Assert, s.lastReason)
		}
	}
	t.Logf("scenario summary: state=%s ready=%v resolvedFrom=%q heldGeneration=%d installs=%d sseFailedOverToSecondary=%v",
		probe.connectionState(), probe.ready(), probe.resolvedFrom(),
		probe.heldGeneration(), probe.configInstallCount(), probe.sseFailedOverToSecondary())
}

// applyFailoverInject maps a failover-rig inject alias to a self-restoring
// toxiproxy action on the primary HTTP leg (or the SSE leg). Each alias carries
// its own duration in ms, after which the fault is cleared by a background
// goroutine — so the failover scenarios need no explicit `clear` event.
func applyFailoverInject(t *testing.T, tp *toxiproxyClient, inj *chaosInject) {
	t.Helper()
	if inj == nil {
		return
	}
	name := inj.Name
	if name == "" {
		name = "primary_fault"
	}
	restoreAfter := func(ms int, fn func()) {
		go func() {
			time.Sleep(time.Duration(ms) * time.Millisecond)
			fn()
		}()
	}
	switch {
	case inj.PrimaryRefusedMs != nil:
		// Disable the primary proxy so its listen port refuses connections.
		tp.setEnabled(t, "http", false)
		restoreAfter(*inj.PrimaryRefusedMs, func() { tp.setEnabled(t, "http", true) })
	case inj.PrimaryHangMs != nil:
		// 'timeout' toxic: accept the TCP connection but never deliver the
		// response, so the fetch blocks (the hang that starves the secondary
		// until InitTimeout without a per-URL deadline).
		tp.addToxic(t, "http", name, "timeout", "downstream", map[string]interface{}{"timeout": *inj.PrimaryHangMs})
		restoreAfter(*inj.PrimaryHangMs, func() { tp.removeToxic(t, "http", name) })
	case inj.PrimaryLatencyMs != nil:
		tp.addToxic(t, "http", name, "latency", "downstream", map[string]interface{}{"latency": *inj.PrimaryLatencyMs})
		restoreAfter(*inj.PrimaryLatencyMs, func() { tp.removeToxic(t, "http", name) })
	case inj.SSEDownMs != nil:
		// Take the SSE leg down while both HTTP legs stay up (f05).
		tp.setEnabled(t, "sse", false)
		restoreAfter(*inj.SSEDownMs, func() { tp.setEnabled(t, "sse", true) })
	default:
		t.Logf("applyFailoverInject: unhandled inject shape %+v — no-op", inj)
	}
}

// ----- rig plumbing -----

func dialRigToxiproxy(t *testing.T) *toxiproxyClient {
	t.Helper()
	toxiURL := envOrDefault("TOXIPROXY_URL", "http://127.0.0.1:8474")
	tp := newToxiproxy(toxiURL)
	if err := tp.ping(context.Background()); err != nil {
		t.Skipf("toxiproxy not reachable at %s: %v — run integration-test-data/chaos/start-chaos.sh first", toxiURL, err)
	}
	return tp
}

// reconfigureRigProxies repoints the seeded proxies at the spawned upstream(s).
// The SSE leg always tracks the primary upstream (failover is HTTP-only).
func reconfigureRigProxies(t *testing.T, tp *toxiproxyClient, host string, primaryPort, secondaryPort int) {
	t.Helper()
	tp.upsertProxy(t, "http", fmt.Sprintf("0.0.0.0:%d", rigPrimaryPort), fmt.Sprintf("%s:%d", host, primaryPort))
	tp.upsertProxy(t, "secondary", fmt.Sprintf("0.0.0.0:%d", rigSecondaryPort), fmt.Sprintf("%s:%d", host, secondaryPort))
	tp.upsertProxy(t, "sse", fmt.Sprintf("0.0.0.0:%d", rigSSEPort), fmt.Sprintf("%s:%d", host, primaryPort))
}

func upstreamGenerations(t *testing.T, ups []chaosUpstream) (primary, secondary int) {
	t.Helper()
	for _, u := range ups {
		switch u.Role {
		case "primary":
			primary = u.Generation
		case "secondary":
			secondary = u.Generation
		}
	}
	return primary, secondary
}

// buildChaosAPIDelivery builds the api-delivery server binary once and returns
// its path. Mirrors startChaosAPIDelivery's build step (GOWORK=off so the
// pinned sdk-go module resolves, not the local sibling).
func buildChaosAPIDelivery(t *testing.T) string {
	t.Helper()
	serverDir := filepath.Join(chaosProjectRoot(), "api-delivery")
	binary := filepath.Join(t.TempDir(), "api-delivery")
	build := exec.Command("go", "build", "-o", binary, "./cmd/server")
	build.Dir = serverDir
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build api-delivery: %v\n%s", err, out)
	}
	return binary
}

// spawnChaosUpstream runs a fixture-mode api-delivery on the given port, pinned
// to the given Meta.generation (FIXTURE_GENERATION), and waits for it to listen.
func spawnChaosUpstream(t *testing.T, binary string, port, generation int) {
	t.Helper()
	fixtureDir := chaosIntegrationDataDir()
	if _, err := os.Stat(fixtureDir); err != nil {
		t.Fatalf("integration-test-data fixtures not found at %s: %v", fixtureDir, err)
	}
	keysPath := chaosFixtureKeysPath()
	if _, err := os.Stat(keysPath); err != nil {
		t.Fatalf("fixture SDK keys not found at %s: %v", keysPath, err)
	}

	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"FIXTURE_DIR="+fixtureDir,
		"SDK_KEYS_FILE="+keysPath,
		"QUONFIG_ENVIRONMENT=development",
		"SSE_HEARTBEAT_INTERVAL=1s",
		fmt.Sprintf("FIXTURE_GENERATION=%d", generation),
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start api-delivery on :%d: %v", port, err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if dialOK("127.0.0.1", port) {
			time.Sleep(100 * time.Millisecond)
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("api-delivery (gen=%d) did not start on :%d within 15s", generation, port)
}

// freePort asks the OS for an unused TCP port and returns it. There is a small
// race between closing the listener and the upstream binding it, but it is
// adequate for spawning per-scenario fixture upstreams in tests.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func globScenarios(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatalf("glob %s: %v", dir, err)
	}
	if len(files) == 0 {
		t.Fatalf("no scenarios found in %s", dir)
	}
	sort.Strings(files)
	return files
}
