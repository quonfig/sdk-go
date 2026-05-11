# Changelog

All notable changes to the Quonfig Go SDK are documented here.

## Unreleased

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

### Deprecated

- `WithRefreshInterval(d)` — preserved as a thin shim over
  `WithFallbackPoll(true, d)`. A one-shot warning is logged at `NewClient`
  time. Callers should migrate to `WithFallbackPoll` to make the
  fallback-only semantic explicit.

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
