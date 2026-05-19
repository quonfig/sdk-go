package quonfig

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// DefaultDataDirAutoReloadDebounce is the default debounce window used to
// coalesce filesystem-event bursts (atomic-rename saves, git pull, etc.).
const DefaultDataDirAutoReloadDebounce = 200 * time.Millisecond

// datadirWatcherConfig wires a datadir-watcher into the rest of the SDK.
// OnChange is invoked once per debounced burst; OnError is invoked for any
// non-fatal watcher error (logger errors, dropped events). The caller owns
// the parse-then-swap step.
type datadirWatcherConfig struct {
	Datadir  string
	Debounce time.Duration
	OnChange func()
	OnError  func(err error)
	Logger   *slog.Logger
}

// datadirWatcher watches a datadir tree and fires OnChange once per debounced
// burst of filesystem events. fsnotify does NOT recurse — we walk the tree at
// Start() and register each subdirectory; new directories created later are
// picked up via the Create event.
//
// Registration failures (read-only filesystem, immutable container, missing
// directory) are surfaced via OnError and Start() returns false; the SDK then
// runs without auto-reload rather than panicking.
type datadirWatcher struct {
	cfg datadirWatcherConfig

	mu       sync.Mutex
	watcher  *fsnotify.Watcher
	debounce *time.Timer
	closed   bool
	closeCh  chan struct{}
	doneCh   chan struct{}
}

func newDatadirWatcher(cfg datadirWatcherConfig) *datadirWatcher {
	if cfg.Debounce <= 0 {
		cfg.Debounce = DefaultDataDirAutoReloadDebounce
	}
	if cfg.OnError == nil {
		cfg.OnError = func(error) {}
	}
	if cfg.OnChange == nil {
		cfg.OnChange = func() {}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &datadirWatcher{
		cfg:     cfg,
		closeCh: make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// Start initializes the underlying fsnotify watcher, resolves any symlinks on
// the configured datadir, walks the tree to register every subdirectory, and
// spawns the event-handling goroutine. Returns false if registration fails;
// the caller should log and continue without auto-reload.
func (w *datadirWatcher) Start() bool {
	w.mu.Lock()
	if w.closed || w.watcher != nil {
		w.mu.Unlock()
		return w.watcher != nil
	}
	w.mu.Unlock()

	// Resolve symlinks once at start. If the user atomically flips the link
	// later we will keep watching the old target — documented behavior, matches
	// sdk-node. Stat to surface ENOENT cleanly before fsnotify masks it.
	resolved, err := filepath.EvalSymlinks(w.cfg.Datadir)
	if err != nil {
		w.cfg.OnError(err)
		return false
	}
	if _, err := os.Stat(resolved); err != nil {
		w.cfg.OnError(err)
		return false
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		w.cfg.OnError(err)
		return false
	}

	if err := addRecursive(fw, resolved); err != nil {
		// Surface the first error and clean up — running with a half-registered
		// tree would mean inconsistent reload behavior.
		_ = fw.Close()
		w.cfg.OnError(err)
		return false
	}

	w.mu.Lock()
	w.watcher = fw
	w.mu.Unlock()

	go w.run(fw)
	return true
}

func (w *datadirWatcher) run(fw *fsnotify.Watcher) {
	defer close(w.doneCh)
	for {
		select {
		case <-w.closeCh:
			return
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			// Auto-add new subdirectories so configs nested under a freshly
			// created directory still trigger reloads (mirrors the git-pull
			// case where a whole tree appears).
			if ev.Op&fsnotify.Create == fsnotify.Create {
				if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
					if addErr := addRecursive(fw, ev.Name); addErr != nil {
						w.cfg.OnError(addErr)
					}
				}
			}
			w.schedule()
		case err, ok := <-fw.Errors:
			if !ok {
				return
			}
			w.cfg.OnError(err)
		}
	}
}

func (w *datadirWatcher) schedule() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if w.debounce != nil {
		w.debounce.Stop()
	}
	w.debounce = time.AfterFunc(w.cfg.Debounce, func() {
		w.mu.Lock()
		if w.closed {
			w.mu.Unlock()
			return
		}
		w.debounce = nil
		fn := w.cfg.OnChange
		w.mu.Unlock()
		fn()
	})
}

// Close stops the watcher goroutine and releases the underlying fsnotify
// handle. Safe to call multiple times.
func (w *datadirWatcher) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	if w.debounce != nil {
		w.debounce.Stop()
		w.debounce = nil
	}
	fw := w.watcher
	w.watcher = nil
	w.mu.Unlock()

	close(w.closeCh)
	if fw != nil {
		_ = fw.Close()
		<-w.doneCh
	}
}

// addRecursive walks the tree rooted at dir and registers every directory
// (skipping hidden directories like .git). fsnotify does not natively recurse
// on Linux/Windows, so we have to enumerate.
func addRecursive(fw *fsnotify.Watcher, dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Best-effort: a transient permission error on a subdir should not
			// abort the whole registration. fsnotify will deliver later events
			// for the dirs we did register.
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		if name := info.Name(); name != "." && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		if err := fw.Add(path); err != nil {
			return err
		}
		return nil
	})
}
