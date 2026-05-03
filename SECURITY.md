# Security Policy

## Reporting a Vulnerability

If you believe you have found a security vulnerability in `github.com/quonfig/sdk-go`, please
report it privately so we can fix it before it becomes public.

**Email:** security@quonfig.com

Please include:

- A description of the issue and the impact you believe it has
- Steps to reproduce, or a proof-of-concept if available
- The version of `github.com/quonfig/sdk-go` you tested against (`go list -m github.com/quonfig/sdk-go`)
- Any relevant configuration (cloud vs. datadir mode, transport options)

We will acknowledge receipt within two business days and aim to provide an initial assessment
within five business days. Please do not file a public GitHub issue, open a pull request that
references the vulnerability, or disclose details on social media or chat until we have published
a fix.

## Supported Versions

We patch the latest published `0.0.x` release. Older versions are not actively maintained — if
you are running one, please upgrade before reporting.

## Scope

In scope:

- The published `github.com/quonfig/sdk-go` module
- Source in this repository (Go packages, `internal/`, build/release config)

Out of scope:

- The Quonfig service itself (api-delivery, app-quonfig). Report those at the same address —
  they are tracked separately.
- Issues in transitive dependencies that are already disclosed upstream and patched in a newer
  version. Please open a regular issue or PR to bump the dependency.
