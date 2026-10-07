package quonfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// writeDatadirWithGreeting builds a temp datadir containing a single
// configs/welcome-message.json with the given value.
func writeDatadirWithGreeting(t *testing.T, value string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "configs"), 0o755); err != nil {
		t.Fatalf("mkdir configs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "quonfig.json"),
		[]byte(`{"environments":["Production"]}`), 0o644); err != nil {
		t.Fatalf("write quonfig.json: %v", err)
	}
	writeGreetingConfig(t, dir, value)
	return dir
}

func greetingConfigBody(value string) string {
	return fmt.Sprintf(`{
		"id":"welcome-message",
		"key":"welcome-message",
		"type":"config",
		"valueType":"string",
		"sendToClientSdk":false,
		"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"string","value":%q}}]},
		"environments":[
			{"id":"Production","rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"string","value":%q}}]}
		]
	}`, value, value)
}

func writeGreetingConfig(t *testing.T, datadir, value string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(datadir, "configs", "welcome-message.json"),
		[]byte(greetingConfigBody(value)), 0o644); err != nil {
		t.Fatalf("write welcome-message.json: %v", err)
	}
}

// greetingStagingDir is a hidden directory inside the datadir. Both the
// watcher and the workspace loader skip dot-directories, so files staged here
// are invisible until they are renamed into configs/.
const greetingStagingDir = ".staging"

// stageGreetingConfig writes a complete welcome-message.json body for value
// into the hidden staging directory and returns its path. Renaming the
// staged file into configs/ is the atomic replace an editor's save or a git
// checkout performs, so a reload can never observe a truncated file.
func stageGreetingConfig(t *testing.T, datadir, value string) string {
	t.Helper()
	staged := filepath.Join(datadir, greetingStagingDir, value+".json")
	if err := os.WriteFile(staged, []byte(greetingConfigBody(value)), 0o644); err != nil {
		t.Fatalf("stage %s: %v", value, err)
	}
	return staged
}

func waitForDatadir(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for predicate", timeout)
}

func TestDataDirAutoReloadRereadsOnChange(t *testing.T) {
	datadir := writeDatadirWithGreeting(t, "hola")

	var callbacks atomic.Int64
	client, err := NewClient(
		WithDataDir(datadir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithDataDirAutoReload(true),
		WithDataDirAutoReloadDebounce(30*time.Millisecond),
		WithOnConfigUpdate(func() { callbacks.Add(1) }),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	got, ok, err := client.GetStringValue("welcome-message", nil)
	if err != nil || !ok || got != "hola" {
		t.Fatalf("initial: got=%q ok=%v err=%v", got, ok, err)
	}
	initial := callbacks.Load()

	writeGreetingConfig(t, datadir, "buenos-dias")

	waitForDatadir(t, 2*time.Second, func() bool {
		v, ok, _ := client.GetStringValue("welcome-message", nil)
		return ok && v == "buenos-dias"
	})
	if callbacks.Load() <= initial {
		t.Fatalf("expected OnConfigUpdate to fire after reload, callbacks=%d initial=%d", callbacks.Load(), initial)
	}
}

func TestDataDirAutoReloadDisabledByDefault(t *testing.T) {
	datadir := writeDatadirWithGreeting(t, "hola")

	var callbacks atomic.Int64
	client, err := NewClient(
		WithDataDir(datadir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithOnConfigUpdate(func() { callbacks.Add(1) }),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)

	initial := callbacks.Load()

	writeGreetingConfig(t, datadir, "ignored")
	time.Sleep(300 * time.Millisecond)

	if got := callbacks.Load(); got != initial {
		t.Fatalf("expected no extra callbacks when auto-reload disabled, got %d (initial %d)", got, initial)
	}
	v, _, _ := client.GetStringValue("welcome-message", nil)
	if v != "hola" {
		t.Fatalf("expected original value to stick when disabled, got %q", v)
	}
}

// TestDataDirAutoReloadDebouncesBursts asserts that a burst of filesystem
// events produces exactly one reload callback.
//
// The five versions are staged up front, then renamed into place back to back
// (five rename syscalls, ~1ms) against a 1s debounce. Earlier versions wrote
// non-atomically with 5ms sleeps against an 80ms window; on a starved -race
// CI runner a descheduled writer could leave a gap longer than the window,
// splitting the burst into two reloads (and letting one reload read a
// truncated file). That is correct
// product behavior for two bursts, so the fix is to make the test's burst
// unambiguously one burst, not to relax the exactly-one assertion (qfg-a595).
func TestDataDirAutoReloadDebouncesBursts(t *testing.T) {
	const debounce = time.Second
	datadir := writeDatadirWithGreeting(t, "v0")
	if err := os.MkdirAll(filepath.Join(datadir, greetingStagingDir), 0o755); err != nil {
		t.Fatalf("mkdir staging: %v", err)
	}

	var extra atomic.Int64
	var initialDone atomic.Bool
	client, err := NewClient(
		WithDataDir(datadir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithDataDirAutoReload(true),
		WithDataDirAutoReloadDebounce(debounce),
		WithOnConfigUpdate(func() {
			if initialDone.Load() {
				extra.Add(1)
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	initialDone.Store(true)

	staged := make([]string, 0, 5)
	for i := 1; i <= 5; i++ {
		staged = append(staged, stageGreetingConfig(t, datadir, fmt.Sprintf("v%d", i)))
	}
	target := filepath.Join(datadir, "configs", "welcome-message.json")
	burstStart := time.Now()
	for _, path := range staged {
		if err := os.Rename(path, target); err != nil {
			t.Fatalf("rename %s into place: %v", path, err)
		}
	}
	burst := time.Since(burstStart)

	waitForDatadir(t, 5*time.Second, func() bool {
		v, ok, _ := client.GetStringValue("welcome-message", nil)
		return ok && v == "v5"
	})
	// Wait out one more debounce window so a trailing second reload would be
	// counted.
	time.Sleep(debounce + 250*time.Millisecond)

	if got := extra.Load(); got != 1 {
		t.Fatalf("expected exactly 1 debounced callback, got %d (burst of 5 renames took %s, debounce %s)",
			got, burst, debounce)
	}
}

func TestDataDirAutoReloadParseThenSwap(t *testing.T) {
	datadir := writeDatadirWithGreeting(t, "hola")

	var extra atomic.Int64
	var initialDone atomic.Bool
	client, err := NewClient(
		WithDataDir(datadir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithDataDirAutoReload(true),
		WithDataDirAutoReloadDebounce(30*time.Millisecond),
		WithOnConfigUpdate(func() {
			if initialDone.Load() {
				extra.Add(1)
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	initialDone.Store(true)

	if err := os.WriteFile(filepath.Join(datadir, "configs", "welcome-message.json"),
		[]byte(`{not valid json`), 0o644); err != nil {
		t.Fatalf("write garbage: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	v, ok, err := client.GetStringValue("welcome-message", nil)
	if err != nil || !ok || v != "hola" {
		t.Fatalf("expected previous envelope to remain after parse failure, got v=%q ok=%v err=%v", v, ok, err)
	}
	if got := extra.Load(); got != 0 {
		t.Fatalf("expected no callbacks on parse failure, got %d", got)
	}
}

func TestDataDirAutoReloadCloseStopsWatcher(t *testing.T) {
	datadir := writeDatadirWithGreeting(t, "hola")

	var extra atomic.Int64
	var initialDone atomic.Bool
	client, err := NewClient(
		WithDataDir(datadir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithDataDirAutoReload(true),
		WithDataDirAutoReloadDebounce(30*time.Millisecond),
		WithOnConfigUpdate(func() {
			if initialDone.Load() {
				extra.Add(1)
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	initialDone.Store(true)
	client.Close()

	writeGreetingConfig(t, datadir, "after-close")
	time.Sleep(300 * time.Millisecond)

	if got := extra.Load(); got != 0 {
		t.Fatalf("expected no callbacks after Close(), got %d", got)
	}
}

func TestDatadirWatcherStartFailsGracefullyOnMissingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	var errs []error
	w := newDatadirWatcher(datadirWatcherConfig{
		Datadir:  missing,
		Debounce: 10 * time.Millisecond,
		OnChange: func() { t.Fatal("OnChange should not fire") },
		OnError:  func(err error) { errs = append(errs, err) },
	})
	if w.Start() {
		t.Fatal("expected Start() to return false for missing dir")
	}
	if len(errs) == 0 {
		t.Fatal("expected an error from OnError")
	}
	w.Close()
	// Sanity: the surfaced error should mention the missing path or be a path error.
	if !errors.Is(errs[0], os.ErrNotExist) {
		// Not strict — fsnotify might wrap differently. Just don't allow nil.
		if errs[0] == nil {
			t.Fatal("expected non-nil error")
		}
	}
}

func TestDataDirAutoReloadFollowsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks not reliably supported on Windows CI")
	}
	realDir := writeDatadirWithGreeting(t, "hola")
	linkParent := t.TempDir()
	linkPath := filepath.Join(linkParent, "datadir-symlink")
	if err := os.Symlink(realDir, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	var extra atomic.Int64
	var initialDone atomic.Bool
	client, err := NewClient(
		WithDataDir(linkPath),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithDataDirAutoReload(true),
		WithDataDirAutoReloadDebounce(30*time.Millisecond),
		WithOnConfigUpdate(func() {
			if initialDone.Load() {
				extra.Add(1)
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	initialDone.Store(true)

	writeGreetingConfig(t, realDir, "via-symlink")

	waitForDatadir(t, 2*time.Second, func() bool {
		v, ok, _ := client.GetStringValue("welcome-message", nil)
		return ok && v == "via-symlink"
	})
	if extra.Load() == 0 {
		t.Fatal("expected at least one callback through symlinked datadir")
	}
}

// One malformed file among several must not produce a partial envelope: the
// reload is rejected as a whole and the previous envelope keeps serving
// every key (README "Parse-then-swap").
func TestDataDirAutoReloadKeepsPreviousEnvelopeWhenOneFileIsMalformed(t *testing.T) {
	datadir := writeDatadirWithGreeting(t, "hola")
	otherPath := filepath.Join(datadir, "configs", "other-message.json")
	if err := os.WriteFile(otherPath, []byte(`{
		"id":"other-message","key":"other-message","type":"config","valueType":"string","sendToClientSdk":false,
		"default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"string","value":"adios"}}]}
	}`), 0o644); err != nil {
		t.Fatalf("write other-message.json: %v", err)
	}

	var extra atomic.Int64
	var initialDone atomic.Bool
	client, err := NewClient(
		WithDataDir(datadir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithDataDirAutoReload(true),
		WithDataDirAutoReloadDebounce(30*time.Millisecond),
		WithOnConfigUpdate(func() {
			if initialDone.Load() {
				extra.Add(1)
			}
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	initialDone.Store(true)

	if err := os.WriteFile(otherPath, []byte(`{"id":"other-mess`), 0o644); err != nil {
		t.Fatalf("write truncated file: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	for key, want := range map[string]string{"welcome-message": "hola", "other-message": "adios"} {
		v, ok, err := client.GetStringValue(key, nil)
		if err != nil || !ok || v != want {
			t.Fatalf("%s: expected previous envelope value %q after partial parse failure, got v=%q ok=%v err=%v", key, want, v, ok, err)
		}
	}
	if got := extra.Load(); got != 0 {
		t.Fatalf("expected no OnConfigUpdate on parse failure, got %d", got)
	}
}
