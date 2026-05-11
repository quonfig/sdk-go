# quonfig

Go SDK for [Quonfig](https://quonfig.com) — Feature Flags, Live Config, and
Dynamic Log Levels.

> **Note:** This SDK is pre-1.0 and the API is not yet stable.

## Installation

```bash
go get github.com/quonfig/sdk-go
```

## Quick Start

```go
import "github.com/quonfig/sdk-go"

client, err := quonfig.NewClient(
    quonfig.WithAPIKey("your-sdk-key"),
)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Feature flag
if on, _, _ := client.GetBoolValue("new-dashboard", nil); on {
    // show new dashboard
}

// Config value
limit, _, _ := client.GetIntValue("rate-limit", nil)
```

## Connection health

The SDK exposes two diagnostic getters so callers can surface freshness in
their own dashboards and alerts:

```go
state := client.ConnectionState()
// One of: Initializing | Connected | Disconnected | FallingBack

last := client.LastSuccessfulRefresh()
// time.Time — zero value before the first install.
```

`ConnectionState` reports the SSE transport state. `LastSuccessfulRefresh`
is the wall-clock time of the most recent installed config envelope, from
either path (SSE push or Layer 2 fallback poll).

### Do NOT wire these into a Kubernetes liveness probe

> Do not wire `LastSuccessfulRefresh()` or `ConnectionState()` directly into a Kubernetes liveness probe. These signals are diagnostic, not pass/fail. A liveness probe based on SDK freshness will amplify transient network blips into restart cascades.

The SDK intentionally does not expose a `Healthy()` primitive. A binary
health signal wired into a liveness probe will reboot pods every time the
SSE stream hiccups — turning a 60-second network blip into a fleet-wide
restart storm. If you need a threshold-based view of staleness, compose it
yourself from `LastSuccessfulRefresh()` (e.g. `time.Since(last) > 10 *
time.Minute`) and surface it as a metric or readiness signal, not a
liveness one.

## Fallback polling

By default the SDK opens an SSE stream and trusts it. To guarantee a
poll-based refresh path during sustained disconnects (≥120s), enable Layer 2:

```go
client, err := quonfig.NewClient(
    quonfig.WithAPIKey("your-sdk-key"),
    quonfig.WithFallbackPoll(true, 60*time.Second),
)
```

The poller is idle while SSE is connected. It only engages after the
stream has been disconnected past `DefaultFallbackPollThreshold` (120s) and
disengages once SSE recovers. While engaged, `ConnectionState()` reports
`FallingBack`.

## See also

- [CHANGELOG.md](./CHANGELOG.md)
- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [SECURITY.md](./SECURITY.md)
