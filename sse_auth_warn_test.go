package quonfig

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// qfg-goi1.2.4: SSE connect hardening.

func sseCaptureLogger() (*slog.Logger, func() string) {
	var buf bytes.Buffer
	var mu sync.Mutex
	h := slog.NewTextHandler(&syncWriter{w: &buf, mu: &mu}, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}
}

func countWarnLines(logs, substr string) int {
	n := 0
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "level=WARN") && strings.Contains(line, substr) {
			n++
		}
	}
	return n
}

// Item 4: a rejected key on the stream (401/403) is logged at WARN once, not
// once per reconnect and not only at Debug. Other statuses stay at Debug.
func TestSSEClientWarnsOnceOnAuthFailure(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	logger, logs := sseCaptureLogger()
	c := newSSEClient(sseClientConfig{
		URL:          srv.URL,
		APIKey:       "bad-key",
		Logger:       logger,
		OnEnvelope:   func(*ConfigEnvelope) {},
		InitialDelay: time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	})
	c.Start()
	if !pollUntil(2*time.Second, func() bool { return hits.Load() >= 4 }) {
		t.Fatalf("only %d connect attempts", hits.Load())
	}
	c.Stop()

	out := logs()
	if got := countWarnLines(out, "401"); got != 1 {
		t.Fatalf("WARN lines mentioning 401 = %d over %d attempts, want exactly 1; logs:\n%s", got, hits.Load(), out)
	}
	if strings.Contains(out, "bad-key") {
		t.Fatalf("log output contains the SDK key:\n%s", out)
	}
}

// After a successful connect the WARN is re-armed, so a key revoked later in
// the process lifetime is reported again.
func TestSSEClientAuthWarnRearmsAfterSuccess(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		switch {
		case n <= 2 || (n >= 4 && n <= 5):
			http.Error(w, "forbidden", http.StatusForbidden)
		case n == 3:
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			writeSSEEnvelope(w, w.(http.Flusher), makeEnvelope("v1", "flag.a", "x"))
		default:
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()

	logger, logs := sseCaptureLogger()
	c := newSSEClient(sseClientConfig{
		URL:          srv.URL,
		APIKey:       "k",
		Logger:       logger,
		OnEnvelope:   func(*ConfigEnvelope) {},
		InitialDelay: time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
	})
	c.Start()
	if !pollUntil(2*time.Second, func() bool { return hits.Load() >= 8 }) {
		t.Fatalf("only %d connect attempts", hits.Load())
	}
	c.Stop()

	out := logs()
	if got := countWarnLines(out, "403"); got != 2 {
		t.Fatalf("WARN lines mentioning 403 = %d, want 2 (once per auth-failure run); logs:\n%s", got, out)
	}
	if got := countWarnLines(out, "503"); got != 0 {
		t.Fatalf("503 was logged at WARN %d times, want 0 (non-auth statuses stay at Debug)", got)
	}
}
