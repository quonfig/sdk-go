---
sdk: sdk-go
commit: 3677aeae549ff157b1051bec3a9246f66b813be7
version: v1.3.0-24-g3677aea
audit_date: 2026-09-28
auditor_model: claude-opus-5-5
auditor_harness: Claude Code (subagent, blind run)
prompt_version: 1
integration_test_data_ref: v2026.05.20 (ac25a0ac8d9611592def8110aa91e93f94f1886a)
overall_grade: B
findings: { critical: 0, high: 1, medium: 4, low: 7 }
---

# sdk-go audit — 2026-09-28

## Verdict
In the default delivery (SDK-key) mode I would run this in production: evaluation
is in-memory with no I/O after init, every getter sits behind a panic boundary,
the reject-older guard is atomic across every network install path, telemetry
is non-blocking and bounded, and `go test -race` (681 tests) is clean. The
biggest risk is in the opt-in datadir mode: one malformed or half-written
config file makes that key vanish and fall back to code defaults, with no log,
which contradicts the README's "keeps serving the previous envelope" contract
(reproduced). In delivery mode the risks are narrower: `Close()` during init
leaks a poller that keeps polling forever, a fast init failure ignores
`ReturnZeroValue`, the SSE dial has no response-header timeout, and a type
mismatch silently returns a zero value with `ok=true`. Confidence is high for the
findings below, because each was reproduced by execution. It is moderate for
overall resilience, because I did not run the chaos rigs.

## Scorecard
| Area | Grade | One-line reason |
|---|---|---|
| Correctness | B | Rule evaluation matches the contract (cross-checked through the real Client), but datadir silently drops malformed keys and type mismatches return zero values with ok=true. |
| Resilience | B | Hedged primary/secondary fetch, poller that also arms for never-connected streams, and panic-guarded workers; but no SSE header timeout and fast-init-failure ignores ReturnZeroValue. |
| Concurrency | B | Race detector clean, no leak across 50 create/close cycles; but Close() racing the init goroutine leaks the supervisor and poller. |
| Security | B | AES-GCM with shape validation, no key logging found, default TLS; error bodies read unbounded; example-context telemetry (raw attribute values) is on by default (documented). |
| Test suite | B | Large, race-clean, fuzzed, generated tests in sync; but the shared contract mostly runs against a parallel internal stack, not the Client runtime path. |
| API design | C | Silent zero-value on type mismatch, one documented option (`WithConfigFetchTimeout`) is a no-op, and init-time blocking of getters is undocumented in the README. |
| Maintainability | B | Heavily commented, small dependency set; but duplicated resolver/evaluator adapters, a dead `internal/transport` package, and a 1455-line `quonfig.go`. |

## What I ran
All in a `git archive HEAD` copy with `GOWORK=off`, go1.26.2 darwin/arm64.

- `git rev-parse HEAD` -> 3677aeae549ff157b1051bec3a9246f66b813be7; `git describe --tags` -> v1.3.0-24-g3677aea
- `go build ./...` -> OK
- `go vet ./...` -> OK
- `gofmt -l .` -> empty
- `go test -race -short -count=1 ./...` without `../integration-test-data` -> `internal/fixtures` FAILS (the fixtures need the sibling data dir; this is expected, not a defect).
- Extracted `integration-test-data` at the CI-pinned tag `v2026.05.20` next to the copy, then reran: all packages ok. JSON count: **681 pass, 0 fail, 5 skip**. The skips are the 5 `TestIntegration_*` tests in `integration_test.go`, which skip under `-short`. (The chaos test also skips under `-short` and when toxiproxy is absent.) No flakes across 2 complete runs with the data present.
- Generator check: `cd integration-test-data/generators && npm ci && npm run generate -- --target=go`, then diffed the 12 regenerated files against the committed `internal/fixtures/*_generated_test.go` -> **no diff**.
- `staticcheck ./...` (local binary) -> clean. `golangci-lint` is not installed, so it was skipped.
- `govulncheck ./...` -> 9 reachable stdlib advisories, all against the local go1.26.2 toolchain (fixed in 1.26.3 to 1.26.6). This is an environment issue, not the code. There is also one non-reachable module advisory in `golang.org/x/sys@v0.13.0` (fixed in v0.44.0).
- `go list -m -u all` -> direct deps current except testify (1.11.1 vs 1.12.1, test-only); `golang.org/x/sys` indirect is pinned at v0.13.0.
- `go test -bench . -benchmem` -> Get* ~570-660 ns/op, 13-15 allocs; targeting eval ~1.3 us; miss 138 ns.
- Reproduction tests: `zz_audit_repro_test.go` in the scratch copy (not in the repo), `go test -race -run TestAudit_ -v .` Results are quoted per finding.
- Chaos rigs: **not run**. Docker was up, but the rig needs a build of the private `api-delivery` as upstream, takes about 18 minutes for smoke, and holds a host-wide lock. I read the scenarios (01-11, http-proxy 08) and `chaos_test.go` instead.
- Earlier reports in `audit/reports/`: **not read**, by instruction for this blind run (see the Comparison section).

## Findings

### [HIGH] Datadir: a malformed config file silently removes that key (at startup and on auto-reload)
- Where: workspace_loader.go:112-127; quonfig.go:314-326; README.md:170 (contract)
- What: `loadWorkspaceConfigs` collects per-file parse errors in `errs` but returns an error only when *no* config loaded (`len(configs) == 0 && len(errs) > 0`). Otherwise the errors are discarded and nothing is logged. On auto-reload, `reloadDatadir` then installs the envelope with the key missing and fires `OnConfigUpdate`. The README states the opposite: "If the new envelope fails to parse (truncated write, mid-`git pull` state, invalid JSON), the SDK logs the error and **keeps serving the previous envelope**."
- Why it matters: a committed merge conflict, a truncated write, or a non-atomic writer turns a flag into `ErrNotFound`, so callers get code defaults. For a kill switch this silently flips behavior. With the file still broken the state persists, and nothing in the logs points to it.
- Evidence (reproduced):
  - Startup: `TestAudit_DatadirMalformedFileSilentlyDropped` -> `NewClient err=nil; Get(bad) ok=false err=config not found; SDK log output=""` (logger at DEBUG).
  - Reload: `TestAudit_DatadirReloadTruncatedFile` -> `b before="2"; after truncated write: value="" ok=false err=config not found; OnConfigUpdate fired 1 times; warn logs containing 'parse'=false`.
- Suggested fix: treat any per-file parse error on reload as a failed parse (keep the previous envelope), and at minimum log each skipped file at WARN. On startup, either fail `NewClient` or log loudly.

### [MEDIUM] Close() during the init fetch leaks the supervisor and fallback poller, which then poll forever
- Where: quonfig.go:720-733 (init goroutine), quonfig.go:766-832 (`startBackgroundWorkers`: no `closeCh` check before `c.sup = sup` / `sup.Start()`), quonfig.go:242-267 (`Close`)
- What: `Close` stops `c.sup` only if it is already set. If `Close` runs while the init fetch is in flight, `startBackgroundWorkers` later installs and starts a supervisor that nobody stops. `startSSE` and `startDatadirWatcher` guard against this with a `closeCh` check; the supervisor path does not. The comment in `Close` says late starters "observe closeCh closed and skip installation", which is false for the supervisor.
- Why it matters: short-lived clients, tests, and apps that shut down during a slow start leak a goroutine that polls `/api/v2/configs` with the SDK key indefinitely. By default it engages after 120s, then polls every 60s.
- Evidence (reproduced, `TestAudit_CloseDuringInitLeaksPoller`, SSE off, poll 50ms): `requests at close+400ms=2, one second later=5, fallbackPoller.Run goroutine alive after Close=true`. By contrast, 50 create/close cycles *after* init completes leaked nothing (`goroutines baseline=6 after 50 create/close=6`).
- Suggested fix: in `startBackgroundWorkers`, check `closeCh` under `c.mu` before assigning/starting the supervisor (mirror `startSSE`), and pass a context cancelled by `Close` to the init fetch.

### [MEDIUM] A fast init failure ignores `OnInitFailure(ReturnZeroValue)` and does not return `ErrInitializationTimeout`
- Where: quonfig.go:550-563, quonfig.go:1169-1182; docs at options.go:465-469 and options.go:17-20
- What: only a `context.DeadlineExceeded` init failure is normalized to `ErrInitializationTimeout`. A fast failure on every leg (503, 401, connection refused) stores the raw transport error. `ReturnZeroValue` applies only when `errors.Is(err, ErrInitializationTimeout)`, so those callers get an error anyway. The `WithConfigFetchTimeout` doc says "if every leg ... errors ... ReturnError makes getters return ErrInitializationTimeout, ReturnZeroValue makes them return zero values."
- Why it matters: a customer who chose `ReturnZeroValue` so that a delivery outage at deploy time cannot fail requests still gets errors in exactly that case. OpenFeature-style consumers see `ErrorCode=GENERAL` instead of `PROVIDER_NOT_READY`.
- Evidence (reproduced, `TestAudit_FastInitFailurePolicy`, server returns 503): `policy=1 (ReturnZeroValue) -> ok=false err=unexpected status 503 ... isInitTimeout=false errorCode=GENERAL` (same for ReturnError).
- Suggested fix: wrap any init failure in `ErrInitializationTimeout` (or a new `ErrInitializationFailed` that `ReturnZeroValue` also honors), and align the docs.

### [MEDIUM] SSE dial has no response-header timeout, so a peer that accepts but never answers wedges the stream forever
- Where: sse_client.go:141-155 (transport clone, no `ResponseHeaderTimeout`, client has no `Timeout`), sse_client.go:250, sse_client.go:273-276 (the read watchdog starts only after headers)
- What: `connectOnce` blocks in `Client.Do` until headers arrive. `ReadTimeout` (90s) arms only after a 200. A proxy or load balancer that accepts the TCP/TLS connection and then never responds holds the SSE goroutine indefinitely, with no reconnect.
- Why it matters: this is mitigated but not harmless. The poller arms because the stream is never "connected", so config stays fresh at the 60s poll cadence after the 120s threshold. But real-time delivery never recovers until the process restarts, and `ConnectionState` stays `initializing`. Chaos scenario 02 injects its stall on an already-open stream, and clears it at 125s, so it would not catch this.
- Evidence (reproduced, `TestAudit_SSEHeaderStallNeverReconnects`, ReadTimeout=200ms): `SSE connect attempts in 3s ...: 1; ConnectionState=initializing`.
- Suggested fix: set `tr.ResponseHeaderTimeout` (for example, 30s), or arm the watchdog timer before `Do`.

### [MEDIUM] Type mismatch returns a zero value with ok=true and no error
- Where: quonfig.go:336-378 (typed getters), config.go:54-117 (`BoolValue`/`IntValue`/... coerce to zero)
- What: `GetBoolValue` on a string config returns `(false, true, nil)`. `GetIntValue` on a non-numeric string returns `(0, true, nil)`. A float is truncated to int silently. `EvaluateDetails` reports no error code. `ErrorCodeTypeMismatch` exists but is produced only for ENV_VAR coercion.
- Why it matters: a config whose type is changed in the UI, or a caller using the wrong getter, silently gets `false`/`0` with `ok=true`. The caller cannot tell that from a real value, so no fallback path is taken. This is "serving wrong values silently" behind a caller or config mistake.
- Evidence (reproduced, `TestAudit_TypeMismatchSilent`): `string config "true": GetBoolValue=(false,true,<nil>) GetIntValue=(0,true,<nil>) EvaluateDetails.ErrorCode=""`.
- Suggested fix: check `val.Type` against the getter and return a wrapped `ErrTypeMismatch` (ok=false). Because this is 1.x, it could be additive: a new sentinel plus an opt-in strict mode.

### [LOW] Panic in a user `OnSSEStateChange` callback crashes the host process
- Where: sse_client.go:503-514 (`dispatchStates` calls `c.cfg.OnStateChange(v)` without recover)
- What: `OnConfigUpdate` and `OnEnvelope` are recovered (quonfig.go:1318-1332, sse_client.go:420-434); the state callback is not, and it runs on an SDK-owned goroutine.
- Why it matters: a bug in customer callback code takes the process down, which is inconsistent with the SDK's own panic-isolation stance. The root cause is customer code, hence Low.
- Evidence (reproduced, `AUDIT_R5=1 go test -run TestAudit_SSEStateCallbackPanicCrashes`): `panic: user callback bug ... (*sseClient).dispatchStates ... sse_client.go:513`, then `FAIL` (the process exited).
- Suggested fix: wrap the call in the same recover/log pattern as `invokeOnConfigUpdate`.

### [LOW] `WithConfigFetchTimeout` is a documented no-op
- Where: options.go:454-478; quonfig.go:145; runtime_transport.go:149-152
- What: the option sets `transport.fetchTimeout`, which only the sequential `FetchConfigs` reads, and nothing but tests calls that. The live path (`FetchConfigsHedged`) uses the hedge delay and abort. The doc promises "a hung primary aborts after this duration."
- Why it matters: little in practice, since the hedge (2s delay, 6s abort) provides failover, but a public knob that silently does nothing misleads tuning.
- Evidence (reproduced, `TestAudit_ConfigFetchTimeoutIgnored`): `WithConfigFetchTimeout(100ms), server takes 1.5s: v="v" ok=true err=<nil> after 1.5s`. Also: `grep -rn 'FetchConfigs(' --include='*.go' . | grep -v _test.go` shows only the definition.
- Suggested fix: deprecate the option in its doc comment, or map it onto `hedgeAbort`.

### [LOW] The shared contract suite mostly exercises a parallel stack, not the production Client path
- Where: internal/fixtures/test_helpers_test.go:14-58, 245-260, 377-390, 401-411, 413-437, 517-560; internal/fixtures/aggregator_helpers_test.go:264-290, 834-879; internal/resolver/resolver.go vs runtime_resolver.go
- What: of about 190 generated cases, all but the datadir, delivery-environment and init-timeout helpers evaluate through `internal/eval` + `internal/resolver`. That is a second adapter and resolver, with its own copies of the `ErrMissingEnvVar`/`ErrUnableToCoerce`/`ErrUnableToDecrypt` sentinels. The Client uses `runtime_eval.go` + `runtime_resolver.go` instead. Telemetry "reason" expectations are checked against reason logic re-implemented in the test harness, and that logic differs from production (`WeightedValueIndex > 0` versus production `IsWeighted`). Duration cases use a test-local ISO-8601 parser, not `ParseISO8601Duration`. The `:return` init-policy check asserts only "not a timeout error", and `assertDefaultStringValue` compares two YAML literals (tautological; currently unused).
- Why it matters: the two stacks can drift without CI noticing. I found no drift today: the durations, missing-env-var and coercion cases run through a real datadir Client agree (`TestAudit_ContractThroughClient` passes). This is a test gap with no present production effect.
- Suggested fix: have the generator drive `quonfig.NewClient(WithDataDir(...))` and `EvaluateDetails`, and delete the twin stack.

### [LOW] Dead and duplicated code
- Where: internal/transport/client.go (no importers anywhere); runtime_transport.go:149-170 (sequential path, test-only); internal/resolver/resolver.go duplicating runtime_resolver.go; internal/eval duplicating runtime_eval.go adapters; `reportableValueFor` duplicated in telemetry.go:21 and internal/resolver/resolver.go:27.
- Why it matters: this adds maintenance cost and confusion for an outside contributor about which path is live. There is no runtime effect.
- Suggested fix: delete `internal/transport` and fold the twins once the fixtures use the Client.

### [LOW] Unbounded read of non-200 response bodies, embedded into caller-visible errors
- Where: runtime_transport.go:292-294; 200 bodies are decoded without a size cap (runtime_transport.go:297)
- What: `io.ReadAll(resp.Body)` on any non-200 status, and the whole body is placed in the error that getters return during init. This is bounded only by `hedgeAbort` time (6s).
- Why it matters: a misbehaving proxy returning a large HTML error page costs memory and produces huge error strings in customer logs. The telemetry path already uses `io.LimitReader(…, 1024)`.
- Suggested fix: `io.LimitReader(resp.Body, 4096)` for the snippet, and an overall cap on the config body.

### [LOW] Getter blocking during init is not documented in the README
- Where: quonfig.go:1348-1396 (`awaitInitialization`); README Quick Start
- What: until the first install, each getter blocks for up to `InitTimeout` (10s default), measured from that call rather than from `NewClient`. The README never mentions this, and the "never block" claim holds only after init.
- Why it matters: a request handler that calls a getter during a cold start with slow delivery can stall for up to 10s.
- Suggested fix: document it next to Quick Start, and consider measuring the deadline from `NewClient`.

### [LOW] Stale indirect dependency with a known advisory
- Where: go.mod:15 (`golang.org/x/sys v0.13.0 // indirect`, via fsnotify)
- What: govulncheck reports an advisory fixed in v0.44.0; it is not reachable from this module's code.
- Suggested fix: `go get golang.org/x/sys@latest && go mod tidy`.

## Vendor claims
| Claim | Verdict | Evidence |
|---|---|---|
| 1.x and follows semver | Partly verified | `git describe` gives v1.3.0-24; README states semver. I did not audit release history (history reading was out of scope for this run). |
| Used by paying customers and api-delivery | Not verified | Outside the repo. |
| Network failure or delivery outage never makes evaluation fail or block; worst case is last-known or customer defaults | Partly verified | After init: `resolveDetailUnsafe` (quonfig.go:548-658) does no I/O, reads a snapshot under RLock, and has a panic boundary (quonfig.go:476-511). A 3-second SSE header stall did not affect Gets. During init: getters block up to InitTimeout (10s) and then return errors under ReturnError; under ReturnZeroValue a fast failure still returns an error (MEDIUM above). Datadir malformed files serve defaults instead of last-known values (HIGH above). |
| Updates are ordered; never replace newer with older | Verified (network paths) | `shouldInstall`/`installEnvelope` (quonfig.go:1223-1307) under `refreshMu` on HTTP, hedge, SSE and poller paths. Gen<=0 is dropped once a real generation is held, and the held generation never decreases. `ordering_guard_test.go`/`guard_rejected_counting_test.go` pass under -race. Caveats by design: datadir bypasses the guard, and an established client can never accept a legitimately lower generation (for example a server-side history rewrite) until restart. |
| Telemetry is best-effort, bounded, never blocks or crashes | Verified | Non-blocking `select`/`default` enqueue with drop (submitter.go `enqueue`); per-window caps (10k/10k/10k/100k seen); retained queue capped at 5 batches/2MB/5min; POST deadline on the request context; `Stop` bounded by a 5s final flush; response body reads limited. Gets do not wait on telemetry. |
| Weighted rollouts normalize by total weight | Verified | evalcore/weighted.go: `threshold = fraction * totalWeight`. |
| Same-generation payload dropped silently; only strictly older counts as guard-rejected | Verified | `isStrictlyOlderThanHeld` (quonfig.go:1256-1263), used at quonfig.go:924 and 1120. |
| Failed telemetry batches resent unmerged with caps, oldest dropped first | Partly verified | Read `serializeWindow` (one serialization, bytes stored and resent) and the `transportQueue` caps. I did not trace every eviction branch in transport_queue.go. |
| CI regenerates contract tests from a pinned tag and fails on diff | Verified | test.yaml pins v2026.05.20; I regenerated locally with zero diff. The chaos workflow pins a different tag (v2026.05.13). |

## Test suite assessment
- Contract coverage (5+ behaviors checked):
  - Weighted consistency (`get_weighted_values`) asserts exact bucket values.
  - Missing env var and coercion (`get_or_raise`) assert sentinel identity, but against the internal twin sentinels.
  - Context precedence (global/block/local merge via `quonfig.Merge`) asserts enabled values.
  - Durations are asserted with a test-local parser.
  - Init-timeout raise uses a real `NewClient`.
  - Datadir environment selection uses a real `NewClient`.
  - I re-ran the durations, missing env var and coercion through the production Client, and they agree.
- Skipped or overridden cases: no `t.Skip` in generated files. The `var _ = ...` block in test_helpers_test.go keeps unused helpers alive, and `assertDefaultStringValue` is tautological.
- SDK behaviors the shared suite does not cover:
  - Hedge/failover ordering, poller engagement, SSE parsing and oversize handling, and Close lifecycle (covered by Go-only unit tests: quonfig_hedge_test, fallback_poller_*, sse_*).
  - Close-during-init, SSE header stall, and datadir partial-parse behavior (covered by nothing; all three are findings).
  - Typed-getter mismatch semantics.
- Tests that can't fail: the `:return` policy check (asserts only the absence of one error type); `assertDefaultStringValue`.
- Timing-based tests: many use sleeps with test hooks (`testFallbackPollThreshold`, `testSSEReadTimeout`). There were no flakes in 2 complete runs.

## What I did not check
- Did not run chaos rigs (`chaos.yaml`, `failover-chaos.yaml`); I read the scenarios only. Scenario 02 would not catch the header-stall wedge.
- Did not read every eviction/Retry-After branch in `internal/telemetry/transport_queue.go`.
- Only skimmed `evalcore/operators.go` (410 lines) and `semver.go`. Regex operators compile per evaluation (operators.go:113), which is linear-time RE2, so no ReDoS, but it costs allocations.
- Did not look at `slog.go`/`QuonfigHandler`, `datadir_watcher.go` internals beyond the reload entry point, or `dev_context.go` beyond confirming it reads `~/.quonfig/tokens*.json` and, by default, injects the developer email into the global context (which then flows into example-context telemetry).
- Did not run golangci-lint (not installed).
- Did not compare behavior against the live api-delivery (for example, whether the generation can ever go backwards server-side).
- Did not read git history, CHANGELOG, or earlier audit reports (blind-run instruction).

## Comparison with earlier audits
Not performed. This run was instructed to be blind: `audit/reports/`,
QUALITY_AUDIT.md, CHANGELOG.md and git history were deliberately not read. A
later reader should do the step-6 diff against the earlier reports.

## How to spot-check this report
1. Datadir silent drop: read workspace_loader.go:112-127 (errors are dropped when any config loaded) against README.md:170. Or place one invalid `.json` under `configs/` of any datadir, call `NewClient(WithDataDir(..), WithEnvironment(..))`, and observe a nil error, the key missing, and no log.
2. Close-during-init leak: in quonfig.go, compare `startSSE` (checks `closeCh` at 889-895) with `startBackgroundWorkers` (818-832, no check).
3. Fast init failure policy: read quonfig.go:1169-1181 (only DeadlineExceeded is normalized) and quonfig.go:550 (ReturnZeroValue requires ErrInitializationTimeout). A 503 test server reproduces it in about 10 lines.
4. SSE header timeout: sse_client.go:141-155 builds the transport with no `ResponseHeaderTimeout`; the watchdog `time.AfterFunc` at :273 runs after `Do` at :250.
5. `WithConfigFetchTimeout` no-op: `grep -rn 'FetchConfigs(' --include='*.go' . | grep -v _test.go` shows only the definition.
