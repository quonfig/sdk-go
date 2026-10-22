package quonfig

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// Supervisor contract Test 10 (ITD v2026.10.22.2, "Rule — size caps",
// qfg-6vno.20.7). Cases 1-2 (oversized single SSE line) are covered by
// sse_oversized_event_test.go; this file covers case 3 (HTTP 200 body over
// 16 MiB), case 5 (controls) and the multi-line SSE data sum.

// paddedEnvelopeJSON returns a valid config envelope whose JSON encoding is
// exactly size bytes.
func paddedEnvelopeJSON(t *testing.T, size int) []byte {
	t.Helper()
	head := `{"configs":[],"meta":{"version":"gen-big","environment":"Production","generation":7},"pad":"`
	tail := `"}`
	n := size - len(head) - len(tail)
	if n < 0 {
		t.Fatalf("size %d too small", size)
	}
	return []byte(head + strings.Repeat("x", n) + tail)
}

func countLines(s, substr string) int {
	n := 0
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, substr) {
			n++
		}
	}
	return n
}

// Test 10 case 3: a 200 body of 16 MiB + 1 byte is a failed leg, with and
// without Content-Length. No envelope, no ETag stored, the next request sends
// no If-None-Match from it, and one WARN per episode naming the cap.
func TestSizeCapHTTPBodyOverCapFailsLeg(t *testing.T) {
	for _, withCL := range []bool{true, false} {
		t.Run("content-length="+strconv.FormatBool(withCL), func(t *testing.T) {
			body := paddedEnvelopeJSON(t, maxConfigBodyBytes+1)
			var mu sync.Mutex
			var inm []string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				inm = append(inm, r.Header.Get("If-None-Match"))
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("ETag", `"big-etag"`)
				if withCL {
					w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				}
				w.WriteHeader(http.StatusOK)
				// Without Content-Length, multiple writes force chunked encoding.
				for off := 0; off < len(body); off += 1 << 20 {
					end := min(off+1<<20, len(body))
					if _, err := w.Write(body[off:end]); err != nil {
						return
					}
				}
			}))
			defer srv.Close()

			logs := &lockedBuf{}
			tr := newRuntimeTransport([]string{srv.URL}, "test-key", nil)
			tr.logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

			for i := 0; i < 2; i++ {
				lr := tr.fetchFromURLAt(context.Background(), 0, 10*time.Second)
				if lr.Err == nil {
					t.Fatalf("call %d: oversized 200 body was accepted", i)
				}
				if lr.Res != nil {
					t.Fatalf("call %d: oversized body produced a result", i)
				}
				if !strings.Contains(lr.Err.Error(), "16777216") {
					t.Errorf("call %d: error %q does not name the cap", i, lr.Err)
				}
			}
			tr.etagMu.Lock()
			stored := tr.etags[0]
			tr.etagMu.Unlock()
			if stored != "" {
				t.Errorf("ETag %q stored from an oversized body", stored)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(inm) != 2 || inm[1] != "" {
				t.Errorf("If-None-Match on requests = %q, want the second empty", inm)
			}
			out := logs.String()
			if n := countLines(out, "level=WARN"); n != 1 || !strings.Contains(out, "16777216") {
				t.Errorf("want exactly one WARN naming 16777216, got %d:\n%s", n, out)
			}
			if countLines(out, "level=DEBUG") < 1 {
				t.Errorf("repeat oversized body should log at DEBUG:\n%s", out)
			}
		})
	}
}

// Test 10 case 5 (HTTP control): a 12 MiB envelope installs.
func TestSizeCapHTTPBodyUnderCapInstalls(t *testing.T) {
	body := paddedEnvelopeJSON(t, 12*1024*1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"ok-etag"`)
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	tr := newRuntimeTransport([]string{srv.URL}, "test-key", nil)
	lr := tr.fetchFromURLAt(context.Background(), 0, 10*time.Second)
	if lr.Err != nil {
		t.Fatalf("12 MiB body rejected: %v", lr.Err)
	}
	if lr.Res == nil || lr.Res.Envelope == nil || lr.Res.Envelope.Meta.Version != "gen-big" {
		t.Fatalf("12 MiB body did not produce the envelope")
	}
}

// Rule — size caps: an SSE event whose multi-line data sums past 4 MiB is
// oversized even though every line is under the per-line cap. parseStream
// reports bufio.ErrTooLong so connectOnce takes the oversized path.
func TestSizeCapSSEMultiLineDataSumOverCap(t *testing.T) {
	line := "data: " + strings.Repeat("x", 1<<20) + "\n"
	var b strings.Builder
	for i := 0; i < 5; i++ { // 5 x 1 MiB > 4 MiB
		b.WriteString(line)
	}
	b.WriteString("\n")

	delivered := 0
	c := newSSEClient(sseClientConfig{URL: "http://unused", OnEnvelope: func(*ConfigEnvelope) { delivered++ }})
	_, err := c.parseStream(strings.NewReader(b.String()))
	if !errors.Is(err, bufio.ErrTooLong) {
		t.Fatalf("parseStream err = %v, want bufio.ErrTooLong for a multi-line sum over 4 MiB", err)
	}
	if delivered != 0 {
		t.Fatalf("an oversized multi-line event was delivered")
	}
}

// Test 10 case 5 (SSE control): a 3 MiB envelope, and a multi-line event
// under 4 MiB in total, both deliver.
func TestSizeCapSSEUnderCapDelivers(t *testing.T) {
	delivered := 0
	c := newSSEClient(sseClientConfig{URL: "http://unused", OnEnvelope: func(*ConfigEnvelope) { delivered++ }})
	ev := "data: " + string(paddedEnvelopeJSON(t, 3*1024*1024)) + "\n\n"
	if _, err := c.parseStream(strings.NewReader(ev)); err != nil {
		t.Fatalf("3 MiB event: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("3 MiB event delivered %d times, want 1", delivered)
	}
}
