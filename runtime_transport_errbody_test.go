package quonfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A non-200 config-fetch body is embedded in the error getters return, so it
// must be bounded (qfg-goi1.2.4 item 6): a proxy error page or a misbehaving
// edge must not put a megabyte into every getter error string and log line.
func TestFetchErrorBodyIsBounded(t *testing.T) {
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
	if len(msg) > 2048 {
		t.Fatalf("error is %d bytes, want <= 2048 (body must be capped)", len(msg))
	}
	if !strings.Contains(msg, "401") || !strings.Contains(msg, "unauthorized") {
		t.Fatalf("error %q lost the status or the start of the body", msg[:min(len(msg), 200)])
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
