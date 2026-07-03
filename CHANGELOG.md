# Changelog

All notable changes to the Quonfig Go SDK are documented here.

## 1.1.1 - 2026-07-03

### Fixed

- **`LastSuccessfulRefresh` now advances on every successful refresh, not only
  on installs (qfg-41nh.11).** A fetch that completes successfully at the HTTP
  layer — 304 Not Modified, or a 200 whose payload the ordering guard rejects
  as equal-or-older — now stamps the refresh time, as does a
  received-and-processed SSE message that was a guard no-op. Previously a
  healthy long-lived client parked on 304s under-reported liveness: the stamp
  froze even though every fetch succeeded. Transport errors still never stamp.
  Two smaller corrections ride along: the initial config fetch now stamps (it
  previously ran before the internal supervisor existed and the stamp was
  lost), and datadir loads/reloads now stamp (any install counts). Additive,
  backward-compatible: the getter's signature and zero-value-before-first-
  refresh contract are unchanged.

## 1.1.0 - 2026-07-01

### Changed

- **Install-guard carve-out for unversioned snapshots (qfg-7h5d.1.16).** A
  delivery payload whose `generation` is absent or `<= 0` (e.g. from a server
  that predates the generation watermark) is installed by an established client
  rather than rejected as older. Defensive back-compat guard — with servers
  that emit true generations it never triggers.
- **HTTP config-fetch now uses a parallel-failover hedge (qfg-7h5d.1.14).** The
  init/refresh fetch fires the primary URL first and, only if it is slow (past a
  hedge delay) or errors, _also_ fires the secondary in parallel — it no longer
  walks the URLs strictly sequentially. Whatever arrives is installed by
  watermark-max (higher `Meta.generation` wins; a late older payload never
  regresses an established client; a late newer payload heals forward). A fast
  healthy primary answers inside the hedge delay, so the secondary stays a cold
  standby and a healthy system adds zero secondary load. The SSE stream is
  untouched and still never fails over.
- Backward-compatible behavioral notes (no promised contract is broken):
  - `ResolvedFrom()` may now return `"primary"` in a both-legs-healthy topology
    where a 1.0.0 client could return `"secondary"`, because the secondary is no
    longer contacted when the primary answers quickly.
  - `OnConfigUpdate` may fire one extra time shortly after `ready()` when a slow
    but newer primary heals forward past the secondary's seed.
  - ETags are now tracked per leg (each URL has its own `If-None-Match` state);
    this fixes a latent cross-leg 304 masking when both legs are read.

### Added

- **`WithConfigFetchHedgeDelay(d)`** — how long the hedge waits for the primary
  before also firing the secondary in parallel (default ~2s). A primary that
  answers within this delay means the secondary is never contacted.
- **`WithConfigFetchHedgeAbort(d)`** — per-leg hard-abort on the hedged path
  (default ~6s). Must exceed the slow-but-alive primary latency you want to heal
  forward from and be `< InitTimeout` (a construction-time warning fires if
  `InitTimeout <= ` this value). `WithConfigFetchTimeout` is unchanged and keeps
  governing the sequential fetch path.

## 1.0.0 - 2026-06-06

### Changed

- **Stable 1.0.0 release.** The Quonfig Go SDK is now declared stable. No API or
  behavior changes from 0.0.29 — this is a coordinated 1.0.0 version stamp across
  the entire Quonfig SDK family.

## 0.0.29 - 2026-06-02

### Changed

- **Dev-context injection is now default-on (qfg-bw7g.3).** `EnableQuonfigUserContext`
  is now a `*bool` tri-state (`nil` = unset). When left unset it defaults to **on**,
  gated solely by the presence of `~/.quonfig/tokens.json`; the loader no-ops without
  that file, so this stays inert in production. Precedence: explicit
  `WithQuonfigUserContext` pointer ?? `QUONFIG_DEV_CONTEXT` env (`true`/`false`) ??
  `true`. Pass `WithQuonfigUserContext(false)` or set `QUONFIG_DEV_CONTEXT=false` to
  opt out. Replaces the prior `applyDevContextEnvOverride` helper.

## 0.0.28 - 2026-05-30

### Changed

- **BREAKING: rename `WithAPIKey` → `WithSdkKey` (qfg-ujcq).** The functional
  option that sets the SDK key is now `WithSdkKey`, matching the naming used by
  every other Quonfig SDK (`sdkKey` / `sdk_key` / `.sdkKey()`) and the
  documentation. `WithAPIKey` has been removed — update call sites from
  `quonfig.WithAPIKey(key)` to `quonfig.WithSdkKey(key)`. The error message on
  an empty key is now "SDK key must not be empty". No behavior change: the same
  `QUONFIG_BACKEND_SDK_KEY` env var is still auto-loaded when no explicit key is
  passed.

## 0.0.27 - 2026-05-29

### Changed

- **Warn when an environment pin is set in delivery mode (qfg-pinh).** In
  delivery (SDK-key) mode the active environment is determined by the SDK key
  on the server, so a `WithEnvironment` / `QUONFIG_ENVIRONMENT` pin is ignored.
  Previously this was silently dropped; `NewClient` now emits a one-time WARN
  at init. Datadir mode (which honors the pin) stays quiet, as does delivery
  mode with no pin. No evaluation behavior change.

### Fixed

- **Report `SPLIT` for a weighted value landing in bucket 0 (qfg-hknp).**
  Reason detection used `WeightedValueIndex > 0`, but the index is a plain
  0-based bucket index defaulting to 0, so a weighted value resolving to bucket
  0 (~half of users on a 50/50 split) was mis-reported as `STATIC`. An explicit
  `IsWeighted` signal now drives the reason; `WeightedValueIndex` stays 0-based
  so telemetry is unchanged.

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
- **Context telemetry wire value renamed `"shapes"` → `"shapes_only"`
  (qfg-6svs).** `ContextTelemetryShapes` now serializes as `"shapes_only"`,
  the value the SDK family agreed on. `WithContextTelemetryMode` still accepts
  the legacy `"shapes"` literal as a deprecated alias for one minor cycle and
  normalizes it to the canonical mode. See
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
