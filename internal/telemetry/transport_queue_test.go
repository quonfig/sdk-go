package telemetry

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func TestClassifyStatus(t *testing.T) {
	cases := map[int]statusClass{
		200: statusOK, 204: statusOK,
		301: statusRejected, 400: statusRejected, 413: statusRejected, 422: statusRejected,
		401: statusAuth, 403: statusAuth, 404: statusAuth,
		408: statusRetryable, 429: statusRetryable, 500: statusRetryable, 503: statusRetryable,
	}
	for status, want := range cases {
		if got := classifyStatus(status); got != want {
			t.Errorf("classifyStatus(%d) = %v, want %v", status, got, want)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"120", 120 * time.Second, true},
		{" 5 ", 5 * time.Second, true},
		{"3600", 10 * time.Minute, true},
		{"99999999999999999999", 10 * time.Minute, true},
		{now.Add(120 * time.Second).UTC().Format(http.TimeFormat), 120 * time.Second, true},
		{now.Add(-time.Hour).UTC().Format(http.TimeFormat), 0, true},
		{"", 0, false},
		{"soon", 0, false},
	}
	for _, c := range cases {
		got, ok := parseRetryAfter(c.in, now)
		if got != c.want || ok != c.ok {
			t.Errorf("parseRetryAfter(%q) = %v, %v; want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// manualNow is a clock whose Now is set by the test; timers are unused here.
type manualNow struct{ now time.Time }

func (c *manualNow) Now() time.Time { return c.now }
func (c *manualNow) AfterFunc(time.Duration, func()) Timer {
	panic("unused")
}
func (c *manualNow) WithTimeout(context.Context, time.Duration) (context.Context, context.CancelFunc) {
	panic("unused")
}

func newTestQueue(clock Clock) *transportQueue {
	return &transportQueue{
		logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
		clock:              clock,
		maxRetainedBatches: 5,
		maxRetainedBytes:   100,
		maxRetainedAge:     5 * time.Minute,
	}
}

// A Retry-After shorter than the 30s floor never shortens it.
func TestSendAllowed_RetryAfterShorterThanFloor(t *testing.T) {
	c := &manualNow{now: time.Unix(1_800_000_000, 0)}
	q := newTestQueue(c)
	b := &retainedBatch{body: []byte("x"), createdAt: c.now}
	q.queue = []*retainedBatch{b}
	q.onRetryableFailure(b, "503", "5")
	c.now = c.now.Add(29 * time.Second)
	if q.sendAllowed() {
		t.Fatal("send allowed 29s after a failure with Retry-After: 5")
	}
	c.now = c.now.Add(time.Second)
	if !q.sendAllowed() {
		t.Fatal("send not allowed 30s after a failure")
	}
}

func TestAppend_EvictsOldestAndByteCap(t *testing.T) {
	c := &manualNow{now: time.Unix(1_800_000_000, 0)}
	q := newTestQueue(c)
	for i := 0; i < 7; i++ {
		q.append([]byte{byte('a' + i)})
	}
	if n, _ := q.counts(); n != 5 {
		t.Fatalf("count = %d, want 5", n)
	}
	if string(q.queue[0].body) != "c" {
		t.Fatalf("head = %q, want c (a, b evicted)", q.queue[0].body)
	}
	// Byte cap: 60 + 60 > 100 evicts the older one.
	q = newTestQueue(c)
	q.append(make([]byte, 60))
	q.append(make([]byte, 60))
	if n, bytes := q.counts(); n != 1 || bytes != 60 {
		t.Fatalf("after byte-cap eviction count/bytes = %d/%d, want 1/60", n, bytes)
	}
	// Oversize never counts against the caps and never evicts.
	q.append(make([]byte, 101))
	if n, _ := q.counts(); n != 2 || !q.queue[1].oversize {
		t.Fatalf("oversize batch not queued as oversize: %d", n)
	}
}

// Age discard is strictly greater than the max age.
func TestExpire_StrictBoundary(t *testing.T) {
	c := &manualNow{now: time.Unix(1_800_000_000, 0)}
	q := newTestQueue(c)
	q.append([]byte("a"))
	c.now = c.now.Add(5 * time.Minute)
	q.expire()
	if n, _ := q.counts(); n != 1 {
		t.Fatal("batch aged exactly 5 min was discarded")
	}
	c.now = c.now.Add(time.Millisecond)
	q.expire()
	if n, _ := q.counts(); n != 0 {
		t.Fatal("batch older than 5 min was kept")
	}
}
