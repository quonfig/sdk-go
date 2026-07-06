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
    quonfig.WithSdkKey("your-sdk-key"),
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

By default the SDK opens an SSE stream and pairs it with a Layer 2 fallback
poller on a 60s interval — matching sdk-node/python/ruby/java. The poller
is idle while SSE is connected; it only engages after the stream has been
disconnected past `DefaultFallbackPollThreshold` (120s) and disengages once
SSE recovers. While engaged, `ConnectionState()` reports `FallingBack`.

Tune the poll cadence with `WithFallbackPoll(true, d)`:

```go
client, err := quonfig.NewClient(
    quonfig.WithSdkKey("your-sdk-key"),
    quonfig.WithFallbackPoll(true, 30*time.Second),
)
```

Opt out entirely (SSE-only, accept silent staleness during outages) with
`WithFallbackPoll(false, 0)`.

## Failover & `QUONFIG_DOMAIN`

By default the SDK derives every hostname from `QUONFIG_DOMAIN` (default
`quonfig.com`):

| Role                     | URL                              |
|--------------------------|----------------------------------|
| Config fetch (primary)   | `https://primary.quonfig.com`         |
| SSE stream (primary)     | `https://stream.primary.quonfig.com`  |
| Config fetch (secondary) | `https://secondary.quonfig.com`       |
| SSE stream (secondary)   | `https://stream.secondary.quonfig.com`|
| Telemetry                | `https://telemetry.quonfig.com`       |

Set `QUONFIG_DOMAIN` to move all of them together (e.g.
`QUONFIG_DOMAIN=quonfig-staging.com`). **Automatic failover and hedging between
the primary and the secondary are on by default** — the secondary runs on
separate infrastructure, and the SDK fails over to it if the primary is
unreachable and hedges to it if the primary is slow.

`WithAPIURLs` replaces the derived list wholesale. To keep automatic failover
with custom URLs, **pass both a primary and a secondary URL**:

```go
client, err := quonfig.NewClient(
    quonfig.WithSdkKey("your-sdk-key"),
    quonfig.WithAPIURLs([]string{
        "https://primary.your-proxy.example",
        "https://secondary.your-proxy.example",
    }),
)
```

A single URL disables failover, and the SDK logs a warning at init. See
[Reliability](https://docs.quonfig.com/docs/explanations/architecture/resiliency)
for the full model.

## Datadir mode: auto-reload on file changes

When you initialize the SDK with `WithDataDir("./path")`, configs are loaded
once from disk at `NewClient` time. Opt in to `WithDataDirAutoReload(true)`
to have the SDK watch the directory and re-read the envelope whenever files
change — an editor save, a `git pull`, or a build step.

```go
client, err := quonfig.NewClient(
    quonfig.WithDataDir("./workspace-data"),
    quonfig.WithEnvironment("development"),
    quonfig.WithDataDirAutoReload(true), // off by default — must be opted in
    quonfig.WithOnConfigUpdate(func() {
        log.Println("Quonfig configs reloaded from disk")
    }),
)
if err != nil {
    log.Fatal(err)
}

// Edit a file under ./workspace-data and OnConfigUpdate fires within ~200ms.

// On shutdown, Close stops the watcher and clears any pending debounce timer.
defer client.Close()
```

### When to enable

- Local development with the datadir checked out from git.
- Self-hosted servers that `git pull` the datadir on a schedule.
- CI jobs that mutate the datadir between assertions.

### When NOT to enable

- **Read-only / immutable filesystems** (some containers, AWS Lambda, scratch
  images). Watch registration may fail; the SDK degrades gracefully (logs the
  error and continues serving the envelope it loaded at `NewClient` time) but
  you are paying for nothing.
- **Build-time-embedded workflows** where the datadir is baked into the
  artifact and never changes at runtime. Watching wastes a file descriptor
  and a goroutine.
- **Production paths where reload timing matters** — e.g. you would rather
  pin the envelope you shipped with and roll forward through a redeploy
  than have it shift under traffic.

Default is `false`; datadir mode is silent until you opt in.

### Behavior contract

- **Parse-then-swap.** If the new envelope fails to parse (truncated write,
  mid-`git pull` state, invalid JSON), the SDK logs the error and **keeps
  serving the previous envelope**. `OnConfigUpdate` is _not_ fired on parse
  failure — only on a successful swap.
- **Debounced.** Bursts of filesystem events (atomic-rename editor saves,
  `git pull` touching dozens of files) coalesce into a single re-read.
  Default window: **200ms** (`DefaultDataDirAutoReloadDebounce`) — long
  enough to absorb the 3–5 events typical editors emit in <50ms, short
  enough that interactive edits feel immediate. Tune via
  `WithDataDirAutoReloadDebounce` if you need a different window.
- **Graceful degrade.** If watch registration fails (read-only fs,
  immutable container, missing directory, EMFILE), the SDK logs via the
  configured `WithLogger` and continues without watching — `NewClient`
  does **not** return an error from a failed registration.
- **Symlinks.** The watcher resolves `DataDir` to its real path at start
  via `filepath.EvalSymlinks`. Editing the file the symlink points at _is_
  detected; atomic flips that retarget the link itself are **not**.
- **Shutdown.** `client.Close()` stops the watcher goroutine, releases the
  underlying fsnotify handle, and clears any pending debounce timer. There
  is no separate handle to manage — the watcher lifecycle is tied to the
  client.

### Tuning the debounce window

```go
client, err := quonfig.NewClient(
    quonfig.WithDataDir("./workspace-data"),
    quonfig.WithDataDirAutoReload(true),
    quonfig.WithDataDirAutoReloadDebounce(1 * time.Second),
)
```

The default (200ms) is tuned for interactive editing. Raise it if you have
a noisy producer (continuously regenerating files) and would rather see one
reload per second than per save. Lower it only if you have measured that
200ms is meaningfully too slow for your use case.

See the [open-source / local how-to](https://docs.quonfig.com/docs/how-tos/open-source-local)
for the cross-SDK story (sdk-node, sdk-go, sdk-ruby, sdk-python, sdk-java).

## See also

- [CHANGELOG.md](./CHANGELOG.md)
- [CONTRIBUTING.md](./CONTRIBUTING.md)
- [SECURITY.md](./SECURITY.md)
