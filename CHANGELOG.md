# Changelog

All notable changes to the Quonfig Go SDK are documented here.

## 0.0.26 - 2026-05-28

### Removed

- **`WithRefreshInterval` deleted (qfg-85wm).** The deprecated functional
  option (a thin shim over `WithFallbackPoll(true, d)` that also logged a
  one-shot deprecation warning) is gone ahead of v1.0.0. Callers must migrate
  to `WithFallbackPoll(enabled, interval)`; the migration is mechanical
  (`WithRefreshInterval(d)` → `WithFallbackPoll(true, d)`,
  `WithRefreshInterval(0)` → `WithFallbackPoll(false, 0)`). See
  `project/plans/sdk-1.0-unification.md` Section 1.

### Changed

- **Layer 2 fallback polling is now ON by default (qfg-wb2n).** A
  `NewClient()` with no explicit `WithFallbackPoll(...)` engages the poller
  on a 60s interval once SSE has been disconnected past the 120s threshold,
  matching sdk-node/python/ruby/java. Previously sdk-go was the family
  outlier — an SSE-only deployment that lost the stream went silently stale.
  The `WithFallbackPoll(enabled, interval)` signature is unchanged; pass
  `WithFallbackPoll(false, 0)` to opt out. See
  `project/plans/sdk-1.0-unification.md` Section 1.

## 0.0.25 - 2026-05-21

CI, test, and dependency only — no SDK runtime or public API changes. Cut to
keep the cross-SDK version matrix aligned.

### Changed

- **CI: `actions/upload-artifact` 4.6.2 → 7.0.1 (#8).** Dependabot bump of the
  GitHub Actions artifact-upload action used by the workflows.
- **Integration tests pinned to `integration-test-data` v2026.05.20 (#10).**
  Bumped the pinned shared-test-data tag and added a guard against stale
  generated tests so the suite only moves when scenarios are deliberately
  rev'd.

### Internal

- Added the generated `datadir_value_type` integration test covering datadir
  int/double value-type coercion (qfg-bwwj).

## 0.0.24 - 2026-05-19

### Added

- **Opt-in datadir auto-reload (qfg-mol-34b).** New `WithDataDirAutoReload(bool)`
  and `WithDataDirAutoReloadDebounce(time.Duration)` options enable filesystem
  watching for the configured `DataDir`. When enabled, the SDK debounces
  filesystem-event bursts (default 200ms) via fsnotify, re-reads the workspace
  via `loadWorkspaceEnvelope`, parses-then-swaps the envelope on success, and
  fires the existing `OnConfigUpdate` callback. Default off. Read-only
  filesystems, missing directories, and other registration failures are
  logged and the SDK continues without auto-reload rather than panicking.
  Symlinked datadirs are resolved at start via `filepath.EvalSymlinks`. New
  dependency: `github.com/fsnotify/fsnotify v1.10.1`.

### Documentation

- **README + godoc for auto-reload (qfg-zx3y.2).** README gains a "Datadir
  mode: auto-reload on file changes" section covering opt-in, when (not) to
  enable, the parse-then-swap / debounce / symlink / graceful-degrade /
  shutdown contract, and a debounce-tuning snippet. Godoc on
  `WithDataDirAutoReload` and `WithDataDirAutoReloadDebounce` expanded to
  surface the same contract via `go doc`. Cross-link to
  docs.quonfig.com/docs/how-tos/open-source-local.

## 0.0.23 - 2026-05-14

CI and release-infrastructure only — no SDK runtime or public API changes.

### Changed

- **Chaos harness wired as a release gate (qfg-47c2.4, qfg-f26e).** The
  cross-SDK chaos harness (toxiproxy + `integration-test-data/chaos`
  scenarios) now runs on every push to `main`, every PR, and on tags via a
  new `chaos.yaml` workflow. `integration-test-data` is pinned to
  `v2026.05.13` so the gate only moves when scenarios are deliberately
  rev'd.
- **Chaos workflow no longer hard-fails on Dependabot PRs.** Dependabot runs
  have no access to repo Actions secrets, so the private `api-delivery`
  checkout (which needs `QUONFIG_REPO_TOKEN`) used to fail the chaos job for
  every dependency-bump PR. The harness steps are now gated on a
  `HAS_REPO_TOKEN` job env var and skipped cleanly when the token is absent;
  the full harness still runs on `main`, tags, and normal PRs.
- **CI action bumps:** `actions/checkout` 4.3.1 → 6.0.2 (#4),
  `golangci/golangci-lint-action` 8.0.0 → 9.2.0 (#3), `actions/setup-go`
  5.6.0 → 6.4.0 (#2).

### Fixed

- Satisfied `errcheck` on `resp.Body.Close` in the SSE TLS ALPN test.

## 0.0.22 - 2026-05-13

### Fixed

- **SSE no longer silently fails against h2-preferring TLS edges (qfg-hpqj).**
  The SSE socket's "force HTTP/1.1" transport setup was incomplete: setting
  `TLSNextProto` to an empty map disabled Go's automatic h2 RoundTripper
  dispatch but did NOT remove `"h2"` from the TLS ALPN advertisement. Against
  Fly's TLS edge (staging and production), which prefers h2, every
  `connectOnce` attempt landed on h2, received raw HTTP/2 frames on a socket
  the transport parsed as HTTP/1, and errored with
  `malformed HTTP response "\x00\x00\x18\x04..."` — silently, since the
  failure path didn't log. The bug was latent in production because all
  existing consumers use `WithFallbackPoll` or the legacy
  `WithRefreshInterval`; SSE-only callers (the new synthetic monitor) hung
  in `ConnectionState() = "initializing"` indefinitely. Fix pins
  `TLSClientConfig.NextProtos = ["http/1.1"]` so ALPN offers only http/1.1.

### Changed

- `connectOnce` now logs `quonfig: SSE connect failed` (and the non-200
  variant) at debug level with the URL and error. Previously failures were
  silent — the qfg-hpqj investigation lost ~30 minutes to that gap.

## 0.0.21 - 2026-05-11

### Changed

- **Polling is now fallback-only, not parallel.** Prior to this release,
  `WithRefreshInterval(d)` ran a background HTTP poll loop in PARALLEL with
  the SSE stream — every interval, regardless of stream health. After this
  release, the new Layer 2 fallback poller is **idle while SSE is connected**
  and only engages after the SSE stream has been disconnected for ≥120s
  (`DefaultFallbackPollThreshold`). Once SSE recovers the poller disengages.
  Net effect: outbound HTTP traffic drops to near-zero in the happy path,
  but freshness during sustained outages is preserved by the fallback path
  (qfg-47c2.20).
- Alpha-phase behavior change — see
  `project/plans/sdk-hardening-and-verification.md` "Phase 3 — Layer 2
  fallback standardization". No semver hold (0.0.x).
- **Example-context telemetry values are now emitted unwrapped on the wire.**
  Previously sdk-go wrapped every property value with a type tag
  (`{"string":"..."}`, `{"int":N}`, etc.) — diverging from sdk-node,
  sdk-ruby, sdk-python, and sdk-javascript, all of which emit `values` as a
  flat map. The wrap broke the search-context UI's property rendering (showed
  as `[object Object]`) and zeroed out the ClickHouse `context_key` column.
  sdk-go now emits `values` as a plain JSON map of property → value, matching
  every other Quonfig SDK (qfg-gcug).

### Added

- `WithFallbackPoll(enabled bool, interval time.Duration)` option — the
  new way to configure Layer 2 polling. Disabled by default; opt in if you
  want the SDK to keep refreshing configs during sustained SSE outages.
- `Client.ConnectionState()` — returns the customer-visible transport state
  (`initializing` | `connected` | `disconnected` | `falling_back`).
- `Client.FallbackPollerActive()` — true while Layer 2 is engaged.
- `Client.LastSuccessfulRefresh()` — wall-clock time of the most recent
  successful config install from any path.
- Startup log line "quonfig: polling configuration" announcing the chosen
  Layer 1/Layer 2 mode and intervals, so deployers can see the new
  fallback-only behavior at boot.
- Internal supervisor pattern: single goroutine per `Client` owning Layer 1
  (SSE) and Layer 2 (fallback poller) workers under `defer recover()`, with
  exponential-backoff restart (500ms → 30s cap) and a new
  `quonfig_sdk_worker_restart_total{layer="<n>",reason="..."}` counter.
  `ConnectionState()`, `FallbackPollerActive()`, and
  `LastSuccessfulRefresh()` are read out of the supervisor (qfg-47c2.17).
- Customer-facing `README.md` documenting `ConnectionState()` /
  `LastSuccessfulRefresh()`, with explicit do-not-wire-into-k8s-liveness
  guidance and rationale for why no binary `Healthy()` primitive is
  exposed (qfg-47c2.22).

### Fixed

- **SSE silent-stall detection.** Added a 90s SSE read deadline (3× the 30s
  server heartbeat), implemented as a `time.AfterFunc` watchdog reset on
  every successful body read; on fire it cancels the per-attempt request
  context and the reconnect path takes over. Also forces HTTP/1.1 on the
  default SSE transport so chaos-test toxiproxy (TCP-only) can observe
  stream stalls. Previously the SDK could serve stale flags forever after a
  NAT timeout or LB half-close, with no log or metric (qfg-47c2.10).
- **Panic in user `OnEnvelope` callback no longer freezes the SDK.** The
  callback invocation is now wrapped in `defer/recover`; on panic the SDK
  logs at ERROR, increments
  `quonfig_sdk_worker_restart_total{layer="1",reason="callback_panic"}`,
  and keeps the SSE loop alive (qfg-47c2.11).
- **`workspace_loader` no longer walks the `schemas/` subdirectory.** The
  datadir loader previously read JSON Schema docs as empty-Key
  `workspaceConfig` rows into the in-memory store. `schemas/` is now
  excluded, matching api-delivery (qfg-uzsl) and sdk-java; empty-Key
  configs are also rejected as defense-in-depth (qfg-r60g).

### Deprecated

- `WithRefreshInterval(d)` — preserved as a thin shim over
  `WithFallbackPoll(true, d)`. A one-shot warning is logged at `NewClient`
  time. Callers should migrate to `WithFallbackPoll` to make the
  fallback-only semantic explicit.

### Internal

- Cross-SDK chaos test harness wired into sdk-go's test runner. Build-tag-
  gated (`go test -tags chaos -run TestChaos`); drives the shared
  `integration-test-data/chaos/` scenarios via toxiproxy. The red-baseline
  scenarios 2 (silent stall), 5 (SSE down), 7 (half-open), and 9 (flapping)
  captured at PR time are all green after the B4/B5 fixes above
  (qfg-47c2.4).

## 0.0.20 - 2026-05-10

### Added

- Public `EvaluationDetails` struct and `Client.EvaluateDetails(key, ctx)` API
  for OpenFeature-shaped evaluation results: `Value`, `Reason`, `ErrorCode`
  (typed enum), `ErrorMessage`, `Variant`, and `FlagMetadata` (qfg-zbz7).
- Typed `ErrorCode` enum with values `FLAG_NOT_FOUND`, `TYPE_MISMATCH`,
  `PROVIDER_NOT_READY`, and `GENERAL`. ErrorCode is set at the actual error
  site in the SDK so consumers (notably openfeature-go) no longer need to
  pattern-match error message text to infer OpenFeature error codes.
- `Variant` and `FlagMetadata` (with `configId`, `configType`, `environment`,
  `ruleIndex`, `weightedValueIndex` keys, camelCase per the cross-SDK spec)
  populated on every `EvaluationDetails`.

### Backward compatibility

- `EvaluateKey(key, ctx)` retains its `(*Value, EvalReason, bool, error)`
  signature and existing semantics (missing flag still returns
  `(nil, ReasonDefault, false, ErrNotFound)` so `errors.Is(err, ErrNotFound)`
  keeps working). New code should prefer `EvaluateDetails`.

## v0.0.19 — 2026-05-07

### Added

- Targeting operators `IS_PRESENT` and `IS_NOT_PRESENT` (qfg-7jnb.4). The
  operators take only `propertyName` (no `valueToMatch`) and resolve the
  (possibly dotted) path against the merged context. A property is "present"
  iff the path resolves AND the resolved value is non-nil; empty string `""`,
  `0`, and `false` are intentionally treated as present. Missing intermediate
  keys in nested context maps count as not present. `IS_NOT_PRESENT` is the
  negation. Required for parity with the other Quonfig SDKs and for
  api-delivery, which embeds this evalcore.
