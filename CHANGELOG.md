# Changelog

All notable changes to the Quonfig Go SDK are documented here.

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
