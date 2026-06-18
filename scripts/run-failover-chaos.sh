#!/usr/bin/env bash
#
# Run the failover + canonical-ordering chaos rigs against sdk-go (bead qfg-7h5d.1.3).
#
# Unlike run-chaos.sh (single upstream), these two rigs spawn their own
# api-delivery fixture upstream(s) from inside the Go test:
#   - TestFailoverChaos drives scenarios-failover/ against ONE upstream behind
#     the primary ('http') + 'secondary' proxies; faults hit the primary leg.
#   - TestOrderingChaos drives scenarios-ordering/ against TWO upstreams pinned
#     to divergent Meta.generations (one per scenario).
#
# So this wrapper only needs to boot toxiproxy; the test repoints the seeded
# 'http'/'secondary'/'sse' proxies at the upstreams it spawns.
#
# Env knobs:
#   CHAOS_RUN          go test -run regex (default 'TestFailoverChaos|TestOrderingChaos')
#   GO_TEST_TIMEOUT    go test -timeout value (default 15m)
#
# Examples:
#   ./scripts/run-failover-chaos.sh
#   CHAOS_RUN=TestFailoverChaos ./scripts/run-failover-chaos.sh   # failover suite only
#   CHAOS_RUN=TestOrderingChaos ./scripts/run-failover-chaos.sh   # ordering suite only

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SDK_GO_DIR="$(cd "$HERE/.." && pwd)"
REPO_ROOT="$(cd "$SDK_GO_DIR/.." && pwd)"
HARNESS_DIR="$REPO_ROOT/integration-test-data/chaos"

if [[ ! -d "$HARNESS_DIR" ]]; then
  echo "chaos harness not found at $HARNESS_DIR — is integration-test-data checked out as a sibling?" >&2
  exit 1
fi

# Identify ourselves to the shared chaos lock (qfg-47c2.32). Owner PID is THIS
# wrapper's pid so the lock survives the whole run, not just the short-lived
# start-chaos.sh subprocess.
export QUONFIG_CHAOS_SESSION="${QUONFIG_CHAOS_SESSION:-sdk-go-failover-$$-$(date +%s)}"
export QUONFIG_CHAOS_OWNER_PID=$$

CHAOS_RUN="${CHAOS_RUN:-TestFailoverChaos|TestOrderingChaos}"
GO_TEST_TIMEOUT="${GO_TEST_TIMEOUT:-15m}"

cleanup_done=0
cleanup() {
  if [[ "$cleanup_done" == "1" ]]; then
    return
  fi
  cleanup_done=1
  echo "==> tearing down chaos harness"
  "$HARNESS_DIR/stop-chaos.sh" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

echo "==> booting toxiproxy via shared launcher (no upstream — the test spawns its own)"
"$HARNESS_DIR/start-chaos.sh"

echo "==> running failover + ordering scenarios ($CHAOS_RUN)"
cd "$SDK_GO_DIR"
GOWORK=off go test -tags chaos -run "$CHAOS_RUN" -timeout "$GO_TEST_TIMEOUT" -v ./...
