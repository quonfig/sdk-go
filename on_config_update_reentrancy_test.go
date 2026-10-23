package quonfig_test

import (
	"sync/atomic"
	"testing"
	"time"

	quonfig "github.com/quonfig/sdk-go/v2"
)

// qfg-eefx: OnConfigUpdate used to run while the install path held refreshMu,
// so a Refresh() from inside the callback re-entered fetchAndInstall and
// blocked forever on the non-reentrant mutex. On the init path that wedged the
// init goroutine before startBackgroundWorkers, so the poller never started.
// options.go documents client calls from the callback as safe; these tests pin
// that contract on the init path and on a later install.

// reentrancyTimeout bounds every wait so a regression fails instead of hanging.
const reentrancyTimeout = 3 * time.Second

func newReentrancyClient(t *testing.T, srvURL string, cb func(c *quonfig.Client)) (*quonfig.Client, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	var clientRef atomic.Pointer[quonfig.Client]
	client, err := quonfig.NewClient(
		quonfig.WithSdkKey("test-backend-key"),
		quonfig.WithAPIURLs([]string{srvURL}),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(true, 50*time.Millisecond),
		quonfig.WithInitTimeout(2*time.Second),
		quonfig.WithOnConfigUpdate(func() {
			calls.Add(1)
			c := clientRef.Load()
			for c == nil {
				time.Sleep(time.Millisecond)
				c = clientRef.Load()
			}
			cb(c)
		}),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	clientRef.Store(client)
	t.Cleanup(client.Close)
	return client, &calls
}

// runBounded runs fn and fails the test if it does not return in time.
func runBounded(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(reentrancyTimeout):
		t.Fatalf("%s still blocked after %s (deadlock: OnConfigUpdate ran under refreshMu)", what, reentrancyTimeout)
	}
}

func TestOnConfigUpdateCanCallRefreshOnInit(t *testing.T) {
	var body atomic.Value
	body.Store(contentEnvelopeJSON(1, false))
	srv := swappableServer(t, &body)

	var cbRefreshReturned atomic.Bool
	var once atomic.Bool
	client, _ := newReentrancyClient(t, srv.URL, func(c *quonfig.Client) {
		if once.CompareAndSwap(false, true) {
			_ = c.Refresh()
			cbRefreshReturned.Store(true)
		}
	})

	runBounded(t, "first getter", func() {
		if contentFlag(t, client) {
			t.Errorf("flag.content = true at init, want false")
		}
	})
	if !pollTrue(reentrancyTimeout, cbRefreshReturned.Load) {
		t.Fatalf("Refresh() called from OnConfigUpdate on the init install never returned")
	}

	// The init goroutine must have gone on to start the fallback poller: a
	// server-side change is picked up without any manual Refresh.
	body.Store(contentEnvelopeJSON(2, true))
	if !pollTrue(reentrancyTimeout, func() bool { return client.HeldGeneration() == 2 }) {
		t.Fatalf("poller never installed gen 2 (held=%d): init goroutine wedged", client.HeldGeneration())
	}
	runBounded(t, "external Refresh", func() { _ = client.Refresh() })
}

func TestOnConfigUpdateCanCallRefreshOnLaterInstall(t *testing.T) {
	var body atomic.Value
	body.Store(contentEnvelopeJSON(1, false))
	srv := swappableServer(t, &body)

	var armed atomic.Bool
	var cbRefreshes atomic.Int64
	client, calls := newReentrancyClient(t, srv.URL, func(c *quonfig.Client) {
		if armed.CompareAndSwap(true, false) {
			_ = c.Refresh()
			cbRefreshes.Add(1)
		}
	})
	runBounded(t, "first getter", func() { _ = contentFlag(t, client) })

	armed.Store(true)
	body.Store(contentEnvelopeJSON(2, true))
	runBounded(t, "Refresh that triggers a re-entrant callback", func() { _ = client.Refresh() })
	if !pollTrue(reentrancyTimeout, func() bool { return cbRefreshes.Load() == 1 }) {
		t.Fatalf("callback's Refresh never returned")
	}
	if !contentFlag(t, client) {
		t.Fatalf("flag.content = false after gen 2 install, want true")
	}
	if calls.Load() < 2 {
		t.Fatalf("OnConfigUpdate calls = %d, want >= 2", calls.Load())
	}
}

func TestOnConfigUpdateCanCallGetterAndClose(t *testing.T) {
	var body atomic.Value
	body.Store(contentEnvelopeJSON(1, true))
	srv := swappableServer(t, &body)

	var seen atomic.Bool
	var closed atomic.Bool
	client, _ := newReentrancyClient(t, srv.URL, func(c *quonfig.Client) {
		v, ok, err := c.GetBoolValue("flag.content", nil)
		if err == nil && ok && v {
			seen.Store(true)
		}
		if closed.CompareAndSwap(false, true) {
			c.Close()
		}
	})
	runBounded(t, "first getter", func() { _ = contentFlag(t, client) })
	if !pollTrue(reentrancyTimeout, seen.Load) {
		t.Fatalf("getter inside OnConfigUpdate never saw the installed value")
	}
}

func pollTrue(d time.Duration, cond func() bool) bool {
	for deadline := time.Now().Add(d); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return true
		}
	}
	return cond()
}

// A panicking OnConfigUpdate is recovered and does not stop later deliveries
// (the drain loop must not stay marked busy after a recovered panic).
func TestOnConfigUpdatePanicDoesNotStopLaterDeliveries(t *testing.T) {
	var body atomic.Value
	body.Store(contentEnvelopeJSON(1, false))
	srv := swappableServer(t, &body)

	client, calls := newReentrancyClient(t, srv.URL, func(c *quonfig.Client) {
		panic("boom from OnConfigUpdate")
	})
	runBounded(t, "first getter", func() { _ = contentFlag(t, client) })
	body.Store(contentEnvelopeJSON(2, true))
	runBounded(t, "Refresh after a panicking callback", func() { _ = client.Refresh() })
	if !pollTrue(reentrancyTimeout, func() bool { return calls.Load() >= 2 }) {
		t.Fatalf("OnConfigUpdate calls = %d after a recovered panic and a new install, want >= 2", calls.Load())
	}
}
