# Contributing to `github.com/quonfig/sdk-go`

Thanks for your interest in contributing! This guide covers the basics of getting set up,
running tests, and sending pull requests.

## Reporting Issues

Before opening a new issue, please check the
[issue list](https://github.com/quonfig/sdk-go/issues) to see if it has already been
reported or fixed.

When filing a bug, include:

- The version of `github.com/quonfig/sdk-go` you're running (`go list -m github.com/quonfig/sdk-go`)
- Go version (`go version`) — we test against Go 1.23
- A minimal reproduction (a snippet, or ideally a failing test) and the actual vs. expected
  behavior

For security issues, please follow [SECURITY.md](./SECURITY.md) instead of filing a public
issue.

## Local Development

The SDK is plain Go with no monorepo tooling. Clone, fetch deps, and you're ready:

```sh
git clone https://github.com/quonfig/sdk-go.git
cd sdk-go
go mod download
```

### Build / Vet

```sh
go vet ./...
go build ./...
```

### Test

```sh
go test -race -short ./...
```

Some tests exercise the integration suite that lives in the sibling
[`integration-test-data`](https://github.com/quonfig/integration-test-data) repo. The CI
workflow checks out both repos side-by-side; for local runs, only the unit-level tests are
required.

## Sending Pull Requests

- Open a draft PR early if you'd like feedback before finishing the implementation.
- Add a test for any behavior change. Bug fixes should include a regression test that fails
  without the fix.
- We follow semver — any breaking change must be called out in the PR description.
- Keep commits focused. If a PR touches both a feature and an unrelated cleanup, split them.

The CI pipeline (`.github/workflows/test.yaml`) runs `go vet ./...` and `go test -race -short
./...` on every push and pull request — please make sure both pass locally before requesting
review.

## Releases

Releases are cut by tagging `vX.Y.Z` on `main`. Releasing is currently maintainer-only; if
your change is ready to ship, leave a note on the PR and a maintainer will cut the release.

Thanks again for contributing!
