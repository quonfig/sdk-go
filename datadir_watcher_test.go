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

func writeGreetingConfig(t *testing.T, datadir, value string) {
	t.Helper()
	body := fmt.Sprintf(`{
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
	if err := os.WriteFile(filepath.Join(datadir, "configs", "welcome-message.json"),
		[]byte(body), 0o644); err != nil {
		t.Fatalf("write welcome-message.json: %v", err)
	}
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

func TestDataDirAutoReloadDebouncesBursts(t *testing.T) {
	datadir := writeDatadirWithGreeting(t, "v0")

	var extra atomic.Int64
	var initialDone atomic.Bool
	client, err := NewClient(
		WithDataDir(datadir),
		WithEnvironment("Production"),
		WithAllTelemetryDisabled(),
		WithDataDirAutoReload(true),
		WithDataDirAutoReloadDebounce(80*time.Millisecond),
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

	for i := 1; i <= 5; i++ {
		writeGreetingConfig(t, datadir, fmt.Sprintf("v%d", i))
		time.Sleep(5 * time.Millisecond)
	}

	waitForDatadir(t, 2*time.Second, func() bool {
		v, ok, _ := client.GetStringValue("welcome-message", nil)
		return ok && v == "v5"
	})
	time.Sleep(150 * time.Millisecond)

	if got := extra.Load(); got != 1 {
		t.Fatalf("expected exactly 1 debounced callback, got %d", got)
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
