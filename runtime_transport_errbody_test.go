package quonfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A non-200 config-fetch body never reaches the error (qfg-9p4i.17.1 D4):
// that error is wrapped by InitError and logged by the poller and supervisor,
// so a proxy error page or an upstream body must not ride along. The status
// is kept.
func TestFetchErrorOmitsBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized " + strings.Repeat("x", 1<<20) + "\n"))
	}))
	defer srv.Close()

	tr := newRuntimeTransport([]string{srv.URL}, "test-key", nil)
	lr := tr.fetchFromURLAt(context.Background(), 0, 5*time.Second)
	if lr.Err == nil {
		t.Fatal("expected an error for a 401")
	}
	msg := lr.Err.Error()
	if len(msg) > 256 {
		t.Fatalf("error is %d bytes, want <= 256 (no body)", len(msg))
	}
	if !strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") {
		t.Fatalf("error %q: want the status and no body", msg[:min(len(msg), 200)])
	}
}

// A short body is trimmed, so a trailing newline does not end up in the error.
func TestFetchErrorBodyIsTrimmed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized) // writes "unauthorized\n"
	}))
	defer srv.Close()

	tr := newRuntimeTransport([]string{srv.URL}, "test-key", nil)
	lr := tr.fetchFromURLAt(context.Background(), 0, 5*time.Second)
	if lr.Err == nil || strings.HasSuffix(lr.Err.Error(), "\n") {
		t.Fatalf("error = %q, want a trimmed body", lr.Err)
	}
}
