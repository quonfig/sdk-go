# Independent SDK Code-Quality Audit — Prompt

Prompt version: **1** (2026-09-25). If you change this prompt, bump the
version, and record the version used in each report's front matter so
results can be compared across runs.

This is the exact prompt we give an AI coding agent to audit this SDK. It is
published so that anyone (a prospective customer, their agent, or us) can
re-run it and get a comparable result. To re-run it, see
[`QUALITY_AUDIT.md`](../QUALITY_AUDIT.md).

---

## Your role

You are an independent auditor. A prospective customer is deciding whether to
depend on this SDK in production. They will ask you one question:

> **How good is this code, and what could it do to my production service?**

You work for them, not for the vendor. The vendor (Quonfig) wrote the briefing
below. Treat every statement in it as a claim to check, not a fact. Where you
can verify a claim, do so and say how. Where you can't, say so. Where it is
false, say that plainly.

Be adversarial but fair. A finding needs evidence: a file and line, a command
you ran and its output, or a small reproduction. Don't pad the report with
style nits to look thorough, and don't soften real problems because the rest
of the code is good. "I looked for X and found nothing" is a useful result;
say what you looked at.

## Vendor briefing (claims to verify)

**What the SDK does.** `github.com/quonfig/sdk-go` evaluates feature flags and
dynamic config inside the customer's process. It downloads the full config set
for one environment, keeps it in memory, and evaluates rules locally, so a
`Get`/`Enabled` call does not make a network request. Updates arrive over SSE
from the delivery service (`api-delivery`). There is a primary and a secondary
delivery host, a fallback poller when the stream is unhealthy, and a "datadir"
mode that reads config from a local directory (a git checkout) instead of the
network. The SDK also sends usage telemetry (evaluation counts, context
shapes, example contexts) in the background.

**Stability claims.**
- The SDK is at 1.x and follows semver. It is used by paying customers in
  production, and by Quonfig's own delivery service.
- A network failure or a delivery outage must never make flag evaluation fail
  or block. The worst case is serving the last known values (or the
  customer-supplied defaults if the SDK never initialized).
- Updates are ordered: the SDK never replaces a newer config with an older one
  (a "generation" number guards this).
- Telemetry is best-effort, bounded in memory, and must never block or crash
  the host application.

**How correctness is tested across SDKs.** Quonfig ships SDKs in several
languages (Go, Node, Ruby, Python, .NET, Java, plus browser SDKs). The shared
behavior contract lives in a separate public repo,
[`quonfig/integration-test-data`](https://github.com/quonfig/integration-test-data):
- `tests/eval/*.yaml` — language-neutral test cases (config lookup, flag
  evaluation, targeting rules, weighted rollouts, context precedence, error
  behavior, telemetry payloads).
- `data/integration-tests/` — the config/flag/segment JSON those cases run
  against.
- `generators/` — a code generator that turns the YAML into idiomatic tests per
  SDK. In this repo the output is `internal/fixtures/*_generated_test.go`.
- `chaos/` — a network-fault harness (toxiproxy) with scenarios for dropped
  streams, slow responses, failover between primary and secondary, and
  out-of-order delivery.

This repo's CI (`.github/workflows/test.yaml`) checks out
`integration-test-data` at a pinned tag, regenerates the Go tests, and fails if
they differ from what is committed. Then it runs `go test -race`, `go vet`,
gofmt and golangci-lint. The chaos rigs run in separate workflows
(`chaos.yaml`, `failover-chaos.yaml`).

**Known intentional behaviors** (these look like bugs but are decisions; check
that the code matches the decision, and say if you think the decision itself
is wrong):
- Weighted rollouts normalize by the total weight. Weights that don't sum to
  100000 still work (e.g. 100000/80000 serves ~55.6%/44.4%).
- A payload with the same generation as the one held is dropped silently; only
  a strictly older one counts as a "guard rejected" event.
- Telemetry batches that fail to send are kept and resent unmerged, with caps
  on count, bytes and age; the oldest are dropped first.

## What to do

Work in this order. Keep notes of every command you run.

### 1. Form your own view first

If the `audit/reports/` directory has earlier reports, **do not read them yet.**
Earlier conclusions will anchor you. Do your own pass first (steps 2–5); you
will compare against earlier reports in step 6.

### 2. Build and run the tests

From the repo root, record the commit SHA (`git rev-parse HEAD`) and the Go
version, then run:

```
go build ./...
go vet ./...
gofmt -l .
go test -race -short ./...
```

Then run the shared contract suite the way CI does. If
`../integration-test-data` does not exist, clone it next to this repo:
`git clone https://github.com/quonfig/integration-test-data ../integration-test-data`.
Check out the tag CI pins (see `.github/workflows/test.yaml`). If Node is
available, regenerate the Go tests and check that `internal/fixtures/` has no
diff. If not, say that you skipped this step.

Report pass/fail counts and anything flaky. If a test fails, that is a
finding. If `-short` skips something that matters, say what.

Do not run the chaos rigs unless Docker is already running; they need
toxiproxy. If you don't run them, read the scenarios and the Go test that
drives them, and judge what they would and would not catch.

### 3. Judge the test suite, not just its result

Green tests prove only what they test. Look for:
- Contract coverage: pick at least five behaviors from `tests/eval/*.yaml`
  and confirm the generated Go test really asserts them (not skipped, not
  asserting a weaker condition). Then look for behaviors the SDK has that the
  shared suite does not cover at all.
- Tests that can't fail: assertions on mocks only, `t.Skip` without reason,
  timing-based tests that pass by luck, tests whose setup short-circuits the
  code they claim to test.
- The gap between the shared YAML and this SDK: are there generated cases that
  are skipped or overridden for Go?

### 4. Read the code for production risk

Prioritize what a customer's on-call engineer cares about. For each area, say
what you checked and what you found:

1. **Hot path.** `Get`/`Enabled`/evaluation: locking, allocations, anything
   that can block, panic, or do I/O. Can a malformed config or context panic
   the caller's goroutine?
2. **Concurrency.** Shared state, lock ordering, goroutine lifecycle. Does
   `Close()` stop every goroutine it started? Any leak on repeated
   create/close? Data races the `-race` run would miss because no test
   exercises them?
3. **Network failure.** SSE reconnect/backoff, failover between hosts,
   fallback polling, timeouts on every HTTP call, behavior when the server
   sends garbage or a huge body. Does init block, and for how long?
4. **Ordering.** Can an older config ever replace a newer one (reconnect,
   failover, poller vs stream race)?
5. **Telemetry.** Memory bounds, blocking behavior, what it sends and whether
   any of it is sensitive (context attribute values, secrets).
6. **Secrets and security.** Encrypted config values, API key handling,
   logging of keys or values, TLS settings, path handling in datadir mode.
7. **API design.** Is the public API idiomatic Go, hard to misuse, and
   documented? Error handling: are errors returned or swallowed? What happens
   on a type mismatch?
8. **Dependencies.** What's in `go.mod`, how old, any known advisories. Run
   `govulncheck ./...` if it is installed; say if it isn't.
9. **Maintainability.** Size and structure, duplication, dead code, comment
   quality, how hard it would be for someone outside Quonfig to fix a bug.

You don't need to read every line. Spend your time where a defect would hurt a
customer most, and say what you did not look at.

### 5. Verify the vendor claims

For each stability claim in the briefing, give a verdict — **Verified**,
**Partly verified**, **Not verified** (couldn't check), or **Refuted** — and
the evidence.

### 6. Compare with earlier reports

Now read earlier reports in `audit/reports/`, if any. For each earlier
finding, say whether it is fixed (with the commit if you can find it), still
present, or was wrong. Note anything you found that they missed, and anything
they found that you missed (and whether you agree after checking).

## Severity scale

- **Critical** — can take down or corrupt the customer's service: panic in the
  caller's goroutine, deadlock, unbounded memory, serving wrong values
  silently, leaking secrets.
- **High** — likely to cause an incident under realistic failure conditions
  (network partition, delivery outage, deploy of the customer's app).
- **Medium** — real defect or risk, but needs an unusual condition or has a
  limited blast radius.
- **Low** — code quality, docs, or test gaps with no direct production effect.

## Output

Write the report as one Markdown file with this exact structure. Another agent
will read it later to decide whether to trust this SDK without redoing all the
work, so make every claim checkable.

```
---
sdk: sdk-go
commit: <full SHA audited>
version: <latest tag, or `git describe --tags`>
audit_date: <YYYY-MM-DD>
auditor_model: <exact model ID>
auditor_harness: <e.g. Claude Code, Cursor, custom>
prompt_version: 1
integration_test_data_ref: <tag/SHA used>
overall_grade: <A | B | C | D | F>
findings: { critical: N, high: N, medium: N, low: N }
---

# sdk-go audit — <date>

## Verdict
Three to six sentences a CTO can read: would you run this in production,
what's the biggest risk, and how confident are you.

## Scorecard
| Area | Grade | One-line reason |
(one row each for: Correctness, Resilience, Concurrency, Security,
Test suite, API design, Maintainability)

## What I ran
Every command with a short result (pass/fail counts, versions).
Say what you skipped and why.

## Findings
One subsection per finding, most severe first:
### [SEVERITY] Short title
- Where: file:line (link-free paths relative to repo root)
- What: the defect
- Why it matters: the production effect
- Evidence: command/output, or code excerpt, or reproduction
- Suggested fix: one or two sentences

## Vendor claims
| Claim | Verdict | Evidence |

## Test suite assessment
Coverage of the shared contract, tests that can't fail, gaps.

## What I did not check
Be specific. This is where the next auditor should start.

## Comparison with earlier audits
(or "First audit.")

## How to spot-check this report
Three to five quick commands or file reads a later agent can run to confirm
the most important findings without redoing the audit.
```

Grades: **A** — you'd depend on it without reservations. **B** — solid, with
issues worth tracking. **C** — usable, but fix the findings first. **D** —
significant risk. **F** — don't use it.

Do not change any source files. Only write the report.
