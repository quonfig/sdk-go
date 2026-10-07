# Changelog

All notable changes to the Quonfig Go SDK are documented here.

## Unreleased

Semver: minor (recommended). Wave 1 local fixes (qfg-goi1.2.4): error-path,
telemetry-only and dead-code changes. No evaluation result on a success path
changes and no exported API is removed. Every fix on its own is patch-level;
the bump is minor only because `WithConfigFetchTimeout` is newly marked
deprecated (semver 2.0.0 item 7). Ship as a patch if that rule is waived.

### Fixed

- **Telemetry no longer keeps references to nested context values
  (qfg-goi1.2.4).** Recording a context for telemetry copied only the top
  level of each named context, so a nested `map[string]interface{}` or slice
  was still the caller's object when the background flush ran `json.Marshal`
  on it. A caller that wrote to that map after the call returned crashed the
  process with `fatal error: concurrent map iteration and map write`. Nested
  `map[string]interface{}`, `[]interface{}` and `[]string` values are now
  deep-copied at record time. Other reference kinds (custom map types, structs,
  pointers) are left out of telemetry (context shapes and example contexts).
  Evaluation is unchanged.
- **ENV_VAR coercion errors no longer contain the env var's value
  (qfg-goi1.2.4).** When an ENV_VAR-provided value could not be converted to
  the config's type, the error quoted the raw value twice: in the message and
  inside the wrapped parse error (`strconv.ParseInt: parsing "..."`, and the
  duration parser's message). Env vars often hold secrets, and this error
  reaches getter callers, logs and OpenFeature error events. The message now
  names the env var, the target type and the config key, plus a reason only
  (`invalid syntax`, `value out of range`, `not valid JSON`, `not a valid ISO
  8601 duration`). `errors.Is(err, ErrUnableToCoerce)` still holds.
- **The SSE connect now gives up on a peer that never sends response headers
  (qfg-goi1.2.4).** The stream's inactivity watchdog was armed only after
  response headers arrived, so a peer that accepted the connection and never
  answered (a half-open load balancer, a stuck proxy) wedged real-time updates
  until `Close`. The SSE transport now sets `ResponseHeaderTimeout` to the SSE
  read timeout (90s by default). It is not shorter because api-delivery sends
  headers with the first event, which on a cold workspace is the 30s
  heartbeat.
- **A rejected SDK key on the SSE stream is logged at WARN (qfg-goi1.2.4).**
  A 401 or 403 from the stream endpoint was logged at Debug only, so a bad or
  revoked key was invisible at the default log level. The first 401/403 now
  logs one WARN (status and URL, never the key); repeats of the same status
  stay quiet until a successful connect re-arms it. Other non-200 statuses
  stay at Debug. Retry behavior is unchanged.
- **A panic in the `WithSSEStateCallback` callback no longer crashes the
  process (qfg-goi1.2.4).** The callback runs on an SDK goroutine with no
  `recover`, unlike `OnConfigUpdate`. Each call is now recovered and logged at
  ERROR with the panic value and stack, and later state edges are still
  delivered.
- **Config-fetch error bodies are capped at 1 KiB (qfg-goi1.2.4).** A non-200
  response body was read in full and embedded in the error that getters return
  (`unexpected status 401 from ...: <body>`), so a large proxy error page
  ended up in every getter error and log line. The body is now read through a
  1 KiB limit and trimmed of surrounding whitespace.
- **The zero value of `ContextSet` is usable (qfg-goi1.2.4).** `var cs
  quonfig.ContextSet; cs.WithNamedContextValues(...)` (or `SetNamedContext`)
  panicked with `assignment to entry in nil map`. Both writers now create the
  map on first use. `NewContextSet` is unchanged.

### Docs

- **README documents dev-context injection (qfg-goi1.2.4).** A new
  "Developer context (`qfg login`)" section says that `NewClient` reads the
  `qfg login` tokens file by default and adds `quonfig-user.email` to the
  global context, that this applies on any machine where someone ran
  `qfg login` (production jobs included), and how to turn it off
  (`WithQuonfigUserContext(false)` or `QUONFIG_DEV_CONTEXT=false`). Behavior
  is unchanged.

### Deprecated

- **`WithConfigFetchTimeout` (qfg-goi1.2.4).** It has had no effect since init
  and refresh moved to the hedged fetch: it set the deadline of the sequential
  fetch path, which nothing called (and which is now deleted, see Removed).
  Its godoc described a per-URL deadline that did not exist. It now carries a
  `Deprecated:` notice pointing at `WithConfigFetchHedgeAbort` (and
  `WithConfigFetchHedgeDelay`). The matching `Options.ConfigFetchTimeout` field
  and `DefaultConfigFetchTimeout` constant are marked deprecated too. All
  three stay, so existing code still compiles; staticcheck will flag uses
  (SA1019).

### Removed

- **Dead code (qfg-goi1.2.4).** Deleted the unused `internal/transport`
  package (nothing imported it; it still had the old store-ETag-before-decode
  bug and accounted for most `govulncheck` call traces) and the unexported
  sequential `runtimeTransport.FetchConfigs` path with its `fetchTimeout`
  field, whose only caller was a test. Init and refresh use the hedged fetch
  (qfg-7h5d.1.14). No exported API changes; `ConfigEvaluator` and
  `ValueResolver` stay exported until the next major.

### Tests

- **Chaos `server_metric(...)` is now SKIPPED with a reason, not a silent 0
  (qfg-goi1.1.1).** The harness used to stub every `server_metric` to 0, so
  `server_metric('quonfig_subscriber_lag_seconds') == 0` passed without
  checking anything (scenarios 01, 02, 04, 06, 07, 11). It now reports
  `SKIP` with the reason: api-delivery exports metrics via OTLP push only,
  there is no scrape endpoint in the rig, and server lag is covered by the
  staging drill qfg-47c2.19 and the `QuonfigSubscriberLagHigh` alert. In a
  compound expression (02) the skipped leaf is neutral and the other leaves
  are still enforced. Each chaos run ends with a "skipped expressions" tally.
  An `sdkMetric` name the probe does not implement now fails loudly instead of
  comparing against 0.
- **De-flaked `TestDataDirAutoReloadDebouncesBursts` (qfg-a595).** It wrote
  five non-atomic replaces 5ms apart against an 80ms debounce, so on a starved
  `-race` runner a descheduled writer could split the burst into two reloads
  (one of them reading a truncated file) and fail with "got 2 debounced
  callbacks, want 1". The test now stages the five versions in a hidden
  directory and renames them into place back to back (~1ms) against a 1s
  debounce. The exactly-one-callback assertion is unchanged; no SDK code
  changed.

### CI

- **Chaos workflows run at the unit ITD pin (qfg-goi1.1.1).** `chaos.yaml`
  moves from `v2026.05.13` and `failover-chaos.yaml` from `v2026.06.19.1` to
  `v2026.10.03`, the same tag `test.yaml` uses. Scenario 05-sse-down now holds
  `lastSuccessfulRefresh` freshness for 60s (qfg-e3ja).
- **Chaos run summaries record the api-delivery SHA.** api-delivery still
  tracks `main` on purpose (the nightly runs exist to catch its drift); both
  chaos workflows now write the resolved SHA and the ITD tag to the job
  summary.

## 1.5.0 - 2026-10-02

### Tests

- **Integration suite drives the public `Client` (qfg-2agi.31).** The
  generated `internal/fixtures` tests now call the typed getters,
  `FeatureIsOn`, `WithGlobalContext` / `WithContext` and the per-call context
  exactly as a customer does, and assert telemetry on the bytes the real
  client flushes on `Close()`. The test-only config loader, evaluator
  (`internal/eval`) and resolver (`internal/resolver`, with its duplicate
  error sentinels) are deleted. A planted bug in the duration getter,
  `FeatureIsOn`, default handling, `WithGlobalContext`, the telemetry reason
  rule, confidential redaction or error propagation now fails the suite. No
  behaviour change.
- **Pin the context-merge rule (qfg-2agi.38).** New tests with disjoint
  attributes in the same named context prove that a newer tier's named
  context replaces the whole same-named context and that contexts it does not
  name survive: global + per-call, global + `WithContext`, nested
  `WithContext`, and the dev `quonfig-user` context merged under a customer
  `WithGlobalContext`. No behaviour change.
- **Integration-test DURATION cases now go through the public
  `Client.GetDurationValue` (qfg-2agi.4).** The fixtures harness used to
  parse the raw resolved string with a test-only copy of the parser and
  allowed +/-1ms, so a green corpus said nothing about the getter customers
  call. Each case now calls `GetDurationValue` and compares the returned
  `time.Duration` exactly against the expected milliseconds.

### Fixed

- **Example contexts without a key are no longer reported (qfg-2agi.31
  review fix).** A context set where no context has a non-empty `key` or
  `trackingId` is now dropped by the example-context telemetry aggregator,
  matching sdk-node. It used to be sent in `example_contexts`.
- **Example contexts identified only by `trackingId` are no longer collapsed
  into one.** The example-context dedup and once-per-hour rate-limit key now
  uses each context's `key`, falling back to `trackingId` (sdk-node's
  `key ?? trackingId`). Context sets that differed only by `trackingId` used
  to share one bucket per context name, so only the first was reported.
- **ENV_VAR-provided `string_list`, `json` and `duration` values now coerce
  to their type (qfg-2agi.21).** A `string_list` variable is split on commas
  with each item trimmed (`"a, b,c"` -> `["a","b","c"]`; an empty variable is
  an empty list); `GetStringSliceValue` used to return `nil`. A `json`
  variable is parsed; `GetJSONValue` used to return the raw string. An
  unparseable `json` variable or a `duration` variable outside the grammar
  now fails resolution with `ErrUnableToCoerce` (absent value, `ok=false`),
  like an uncoercible `int`, `double` or `bool`.
- **Duration parsing now follows the shared Quonfig grammar exactly
  (qfg-2agi.13).** `ParseISO8601Duration` accepts only
  `P[nD][T[nH][nM][n[.fff]S]]`: at least one component, no dangling `T`, a
  fraction only on seconds (at most 9 digits), ASCII digits, and a magnitude
  of at most `P36500D`. It used to accept `PT` (as 0), `P1DT`, years, months
  and weeks, out-of-order and repeated units, `.5`/`5.`, trailing junk after
  a number (`P1.2.3D` == `P1.2D`), and overflowed silently past ~292 years.
  Fractional seconds now use exact decimal arithmetic rounded half up to
  whole milliseconds instead of float math.
- **`GetDurationValue` reports a malformed duration as absent with a
  coercion error (qfg-2agi.13).** A stored or ENV_VAR-provided value outside
  the grammar now returns `(0, false, err)` with `errors.Is(err,
  ErrUnableToCoerce)`, instead of `ok=true` with an untyped parse error (or,
  for values like `PT0.5H` and `P1DT`, a silently accepted value).
- **Datadir auto-reload no longer drops a key when one config file is
  malformed (qfg-9dxb.10).** A truncated or invalid file used to be skipped
  while the rest of the reload was installed, so that key silently fell back
  to code defaults and `OnConfigUpdate` fired. Now any unreadable file
  rejects the whole reload: the SDK logs a warning and keeps serving the
  previous envelope, as the README promises. At startup a bad file is still
  skipped so the client can boot, but it is now logged instead of dropped
  silently.
- **`Close()` during the initial fetch no longer leaks background workers
  (qfg-9dxb.10).** The supervisor and fallback poller started after the
  fetch returned and kept polling with the SDK key forever. They now see the
  client is closed and never start. This also fixes a data race between
  `Close()` and startup.

## 1.4.0 - 2026-09-28

### Changed

- **Behavior change: a weighted rollout that hashes on a property missing
  from the context now hashes an empty value (qfg-9dxb.8).** Every caller
  without a value for the property gets the same variant for that flag, the
  same one a caller with the property set to `""` gets. This covers a call
  with no context, a context without the named context (for example no
  `user`), a named context without the property, and a property set to nil.
  Before, a caller with no value got a random variant on every evaluation, so
  the same caller could see a different value on every call. A caller whose
  property was set to nil already got a fixed variant in 1.3.0. That variant
  may now change to a different one, once, when you upgrade. A variant with
  weight 0 is not served to these callers. The reason stays `SPLIT`, and `EvaluateDetails` adds
  `hashPropertyMissing: true` to `FlagMetadata` only when the property is
  missing (not when it is present and empty). The SDK logs one warning per
  flag per client. When the property is present, every user lands in the
  same variant as in 1.3.0. A rollout with no hash property configured still
  picks a random variant on every evaluation, unchanged.

### Added

- **Example contexts are sent at most once per hour per context
  (qfg-cg1e).** With the default `periodic_example` context telemetry, the
  Go SDK used to re-send every distinct context it saw in every 60-second
  window, so a busy service sent up to 60 times more example-context data
  than the other Quonfig server SDKs. Now each context is sent once and not
  again for an hour, matching those SDKs, so example-context telemetry
  volume drops. The SDK remembers up to 100,000 recently sent contexts for
  this; set the limit with the new `WithTelemetryMaxExampleContextsSeen`
  option. When the limit is full, a new context is not remembered and is
  picked up on a later evaluation once older entries expire. Evaluations and
  targeting are not affected.

### Fixed

- **A stream that connects and drops at once no longer leaves the SDK
  believing it is connected.** SSE connected/disconnected changes were
  delivered on separate goroutines, so "disconnected" could arrive before
  "connected". The SDK then reported `ConnectionState()` as `connected`, the
  fallback poller never started, and config stopped updating for the whole
  outage. Changes are now delivered in order, still off the stream reader,
  including to a `WithSSEStateCallback` callback. A slow or blocked
  `WithSSEStateCallback` callback can no longer delay the SDK's own failover
  to polling: `ConnectionState()` and the fallback poller update without
  waiting for it.
- **Concurrent percentage-rollout evaluation no longer races or panics
  (qfg-9dxb.1).** A weighted rollout that picks a random variant shared one
  unsynchronized `*rand.Rand` across every goroutine. Under concurrent `Get*`
  calls this was a data race that could panic with `index out of range [-1]`
  and crash the process. The random source is now mutex-guarded. After this
  release, only a rollout with no `hashByPropertyName` configured picks a
  random variant, and the fix covers that path.
- **A panic during evaluation no longer crashes the process (qfg-9dxb.1).**
  Every `Get*`/`EvaluateKey`/`EvaluateDetails` call now recovers a panic raised
  while evaluating a config, logs it at ERROR with a stack trace, and returns
  the caller's default with an error (`Reason: ERROR`, `ErrorCode: GENERAL`).
- **Malformed config no longer crashes the process (qfg-9dxb.4).** Three
  inputs used to kill every process evaluating the affected key: an `IN_SEG`
  reference cycle (a segment that is, directly or via A->B->A, in itself)
  and a `decryptWith` cycle each recursed into an unrecoverable
  `fatal error: stack overflow`, and ciphertext shorter than the 16-byte GCM
  tag panicked with a slice-bounds error. A segment cycle now evaluates like a
  missing segment (`IN_SEG` false, `NOT_IN_SEG` true); a `decryptWith` cycle
  and short ciphertext / empty IV return `ErrUnableToDecrypt`.
- **Fallback poller now engages when SSE never connects, and immediately when
  SSE is disabled (qfg-9dxb.2).** Before, the Layer 2 poller armed its 120s
  engage timer only on a connected-to-disconnected edge. A stream that never
  connected (primary outage at app start, a proxy that breaks streaming) or
  `WithSSE(false)` left config frozen at the init snapshot and
  `ConnectionState()` stuck at `initializing`. The timer is now armed when the
  poller starts, and with `WithSSE(false)` the poller engages at once, as the
  `WithSSE` doc already promised: the SDK polls every 60 seconds (the default
  interval) from startup, and `ConnectionState()` reports `falling_back`
  while it polls.
- **A non-envelope response no longer wipes the config (qfg-9dxb.3).** A 200
  (or SSE event) whose body is not a config envelope, such as `{}` or a
  proxy's `{"error":...}`, used to install as an empty config and every key
  fell back to its default. A payload now needs a `meta` object with a
  non-empty `version`. Over HTTP a failed check counts as a leg error, so the
  hedge and failover run as they would for a 5xx, and the response's ETag is
  stored only after the body passes. An invalid SSE event is dropped, the same
  way malformed JSON is. A payload with a version but no generation still
  installs on a client that has never received a real generation.
- **A generation-0 payload no longer replaces the config of a client that
  holds a real generation, and `HeldGeneration()` never goes backward
  (qfg-9dxb.3, qfg-9dxb.9).** Once the SDK has config with a real generation,
  it ignores a payload without one (sent only by a damaged server). It keeps
  its current config until a normal update arrives. Before, it installed such
  a payload, so it could briefly move back to old config, and
  `HeldGeneration()` dropped to 0, which let the next older snapshot move it
  further back. A client that has never received a real generation still
  installs every such payload. The ignored payload is not counted as
  `guardRejected`.
- **An SSE event larger than 4 MiB no longer causes a reconnect storm or
  frozen config (qfg-9dxb.6).** Before, once a workspace's config payload
  grew past 4 MiB, the SDK silently dropped it, reconnected about three
  times per second, and kept reporting the stream as connected, so the
  fallback poller never took over and config stopped updating. Now the SDK
  logs a WARN, backs off between reconnects as it does for any failed
  connection, reports the stream as disconnected, and the fallback poller
  keeps config current over HTTP. When a normal-size event arrives again,
  streaming resumes.
- **One malformed config no longer blocks the whole workspace
  (qfg-9dxb.6).** Before, if any single config held a value the SDK could
  not decode (for example a `json` value saved as a string, or an `int` of
  `"12a"`), the SDK rejected the entire payload: a new process failed to
  initialize, and running processes stopped receiving updates, with no log on
  the streaming path. Now the SDK skips just that config, logs a WARN naming
  its key, and loads everything else. The warning names the key but never
  prints the value. The skipped key behaves as not found (your default is
  returned) until a decodable version is published. If every config in a
  payload fails to decode, the payload is still rejected as a whole, so it
  can never wipe your config.
- **Changing a list or JSON value you got from the SDK no longer changes
  config for the rest of your process (qfg-9dxb.6).** `GetStringSliceValue`,
  `GetJSONValue`, `EvaluateKey` and `EvaluateDetails` used to return the
  SDK's own copy of list and JSON-object values. If your code changed the
  result (for example sorted a list or added a key to a map), every later
  read in the process saw the change, and two goroutines changing a map at
  the same time could crash the process with `concurrent map writes`. Each
  call now returns its own copy, which you are free to change. Scalar values
  are unaffected. Each call copies the value; for large JSON values read per
  request, keep the result instead of calling repeatedly.

## 1.3.0 - 2026-09-25

### Changed

- **Telemetry transport policy (qfg-y8je.6).** The telemetry POST deadline goes
  from 30s to 15s, and it now rides the request context, so a client passed via
  `WithHTTPClient` (which used to replace the 30s timeout entirely) can no longer
  remove it. The SDK's own telemetry client adds a 5s connect/TLS timeout. The
  5x immediate retry with exponential backoff (which also retried 4xx) is
  removed: a failed batch is kept byte-for-byte and resent (no merging, so the
  server dedups a resend of a batch that did land). Resends happen no sooner
  than 30s after a failure and honor `Retry-After` up to 10 min. The retained
  queue is capped at 5 batches / 2MB / 5 min (oldest dropped; a batch larger
  than the byte cap is never kept). At most one POST is in flight.
  401/403/404 disable telemetry for the process with one ERROR; any other 4xx
  drops that batch with one ERROR.
- **Telemetry now logs, through `WithLogger`.** Before, the telemetry package
  had no logger and every failure was invisible. Now: a failed POST logs at
  debug, one WARN when data is actually dropped (then a summary at most every
  10 min), one INFO on recovery, one ERROR on an auth failure.
- **`Close()`** sends the live telemetry window once with a 5s deadline and does
  not resend kept batches; it returns within that deadline even if the
  endpoint hangs (before: up to 5 attempts of up to 30s each, with backoff).
- **Memory caps:** evaluation-summary counters, context-shape fields and example
  contexts are each capped at 10,000 per window (before: uncapped). Counters
  already present keep counting at the cap; new keys beyond it are dropped.

### Added

- New options (all optional; zero values in `Options` mean the default):
  `WithTelemetryTimeout` (15s), `WithTelemetryConnectTimeout` (5s),
  `WithTelemetryMaxRetainedBatches` (5), `WithTelemetryMaxRetainedBytes`
  (2097152), `WithTelemetryMaxRetainedAge` (5 min),
  `WithTelemetryMaxEvaluationSummaries` (10000),
  `WithTelemetryMaxContextShapeFields` (10000),
  `WithTelemetryMaxExampleContexts` (10000), the matching `Options` fields and
  `DefaultTelemetry*` constants.

Unchanged: the flush interval (`WithTelemetrySyncInterval`, 60s) and the
`ContextTelemetryMode` default (`ContextTelemetryPeriodicExample`). No wire
change, no removed or changed public API, no new dependencies.

## 1.2.2 - 2026-09-14

### Fixed

- **Only a STRICTLY older payload counts as `guardRejected` telemetry
  (qfg-rr5b, qfg-alnb).** The reject-older ordering guard drops any snapshot
  that does not advance the held generation, and every drop was reported as
  `guardRejected` on the failover telemetry event. But two server behaviours
  legitimately re-deliver the envelope the client already holds, at the SAME
  generation: api-delivery re-sends the current envelope on every SSE connect,
  and a config poll whose per-leg ETag slot is cold (a fresh transport, a
  reconnect, the fallback poller's engage-time fetch) answers a full 200
  instead of a 304. Counting those made `guardRejected` non-zero for perfectly
  healthy clients, polluting the `sdk_failover` signal where the field is
  supposed to mean "a leg tried to move us backwards". An equal-generation
  re-delivery is now a silent no-op on both install paths (HTTP fetch and SSE):
  still not installed, still advancing `LastSuccessfulRefresh` exactly where it
  did before, but no longer counted. A strictly older payload is still counted,
  and the `generation <= 0` unversioned carve-out (such a snapshot installs and
  is never a rejection) is unchanged.

  **Reading the field: `guardRejected` reports LOWER numbers on 1.2.2 than on
  1.2.1 for the same traffic** — a steady-state client that previously reported
  one or more per SSE reconnect and per cold-ETag poll now reports zero. That
  is the fix, not a regression: the drop is the equal-generation re-deliveries
  leaving the count. Nothing else changed — no wire, ClickHouse, or dashboard
  change, no public API change, no new dependencies. Decided cross-SDK
  (2026-09-11) and shipping in all six backend SDKs; sdk-ruby 1.4.1 and
  sdk-python 1.4.1 shipped the same narrowing.

## 1.2.1 - 2026-08-11

### Fixed

- **Telemetry is no longer submitted without an SDK key (qfg-j001).**
  `Options.TelemetryEnabled()` gated only on the telemetry URL and the
  collector switches, never on `APIKey`, so a client with no SDK key still
  started the submitter. On the open-source / no-account path — a datadir-only
  client, which needs no key — every flush POSTed to the telemetry endpoint
  with `Authorization: Basic base64("1:")`, an unauthenticated request the
  backend rejects and the submitter then retries with backoff. The gate now
  keys off SDK-key presence rather than off the mode, matching sdk-node.
  A datadir client **with** a key still emits exactly as before (the dogfood
  path used by app-quonfig and api-telemetry is unaffected), and telemetry
  from any keyed client is unchanged.

## 1.2.0 - 2026-07-08

### Added

- **Warning when an explicit `WithAPIURLs` disables failover (qfg-41nh.26).**
  The default (and every `QUONFIG_DOMAIN`-derived) API-URL list carries a
  primary and a secondary leg, and the SDK hedges/fails over between them. An
  explicit `WithAPIURLs` with a single entry silently dropped the secondary;
  the SDK now logs a one-line WARN at init pointing the caller at the fix (pass
  both a primary and a secondary URL). Behavior is otherwise unchanged; no new
  dependencies. New README section documents the `QUONFIG_DOMAIN` derivation
  and the failover model.
- **Failover telemetry emission (qfg-41nh.18).** The SDK now emits a
  per-flush-window `failover` telemetry event carrying failover-behavior
  counters — hedge-fired, guard-rejected, and resolved-from
  (primary/secondary) — on the existing telemetry wire, so failover dashboards
  (qfg-41nh.19) can be built. Additive and opt-out-respecting: the counters ride
  any already-enabled telemetry stream, a healthy steady-state client emits
  nothing (every counter zero, no event sent), and an older api-telemetry
  silently ignores the new event. No new dependencies.

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
