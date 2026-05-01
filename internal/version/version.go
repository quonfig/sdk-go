// Package version exposes the SDK module version for telemetry and HTTP
// headers.
//
// The version is read from runtime/debug.ReadBuildInfo so it reflects the
// version a consumer actually pulled in (e.g. v0.0.19), not a constant the
// SDK author forgot to bump. When the SDK is being developed in-tree (its
// own tests, replace directives, or unsubmitted commits) ReadBuildInfo
// reports "(devel)" — in that case we fall back to a known-recent string
// so logs stay parseable.
package version

import (
	"runtime/debug"
	"sync"
)

const modulePath = "github.com/quonfig/sdk-go"

// fallback is used when running in-tree (go test, replace directives) where
// debug.ReadBuildInfo returns "(devel)". Bump in lockstep with `git tag`.
const fallback = "0.0.0-devel"

var (
	cached     string
	cachedOnce sync.Once
)

// Get returns the bare semver of the imported sdk-go module (no "go-"
// prefix). Examples: "0.0.19", "0.0.0-devel".
func Get() string {
	cachedOnce.Do(func() {
		cached = lookup()
	})
	return cached
}

// Header returns the value sent in X-Quonfig-SDK-Version, e.g. "go-0.0.19".
func Header() string {
	return "go-" + Get()
}

func lookup() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fallback
	}
	for _, dep := range info.Deps {
		if dep != nil && dep.Path == modulePath && dep.Version != "" && dep.Version != "(devel)" {
			return trimV(dep.Version)
		}
	}
	if info.Main.Path == modulePath && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return trimV(info.Main.Version)
	}
	return fallback
}

func trimV(v string) string {
	if len(v) > 0 && v[0] == 'v' {
		return v[1:]
	}
	return v
}
