# Changelog

All notable changes to the Quonfig Go SDK are documented here.

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
