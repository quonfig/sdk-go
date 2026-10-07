package quonfig

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// qfg-goi1.2.4: SSE connect hardening.

// Item 3: a peer that accepts the TCP connection but never sends response
// headers must not wedge the stream. The inactivity watchdog is only armed
// after Do returns, so before the fix the client sat in Do forever and never
// reconnected. ResponseHeaderTimeout = HeaderTimeout bounds it.
func TestSSEClientReconnectsWhenPeerNeverSendsHeaders(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	var accepts atomic.Int32
	var connsMu sync.Mutex
	var conns []net.Conn
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			accepts.Add(1)
			connsMu.Lock()
			conns = append(conns, conn) // hold it open, never answer
			connsMu.Unlock()
		}
	}()
	defer func() {
		connsMu.Lock()
		for _, c := range conns {
			_ = c.Close()
		}
		connsMu.Unlock()
	}()

	c := newSSEClient(sseClientConfig{
		URL:           "http://" + ln.Addr().String() + "/sse",
		APIKey:        "k",
		OnEnvelope:    func(*ConfigEnvelope) {},
		HeaderTimeout: 200 * time.Millisecond,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      20 * time.Millisecond,
	})
	c.Start()
	defer c.Stop()

	if !pollUntil(2*time.Second, func() bool { return accepts.Load() >= 2 }) {
		t.Fatalf("accepts = %d after 2s, want >= 2: the client never gave up on a peer that sends no headers", accepts.Load())
	}
}

func TestSSEClientDefaultTransportHeaderTimeoutIs90s(t *testing.T) {
	for _, rt := range []time.Duration{0, 5 * time.Second} { // default and the chaos-compressed read deadline
		c := newSSEClient(sseClientConfig{URL: "http://x", OnEnvelope: func(*ConfigEnvelope) {}, ReadTimeout: rt})
		tr := c.cfg.Client.Transport.(*http.Transport)
		if tr.ResponseHeaderTimeout != 90*time.Second {
			t.Fatalf("ReadTimeout=%v: ResponseHeaderTimeout = %v, want 90s (api-delivery may hold headers until the 30s heartbeat; must not shrink with ReadTimeout)", rt, tr.ResponseHeaderTimeout)
		}
	}
}

// qfg-d1o9: the header budget must not shrink with the inactivity deadline.
// Path latency adds to the time-to-headers but not to the gaps between
// stream bytes, so a slow-but-live stream that the read watchdog tolerates
// must still be able to connect. Before the fix ResponseHeaderTimeout was
// ReadTimeout, so a header delay longer than ReadTimeout looped the client
// forever (chaos 03-latency: 5s latency vs the harness's 5s read deadline).
func TestSSEClientConnectsWhenHeadersArriveAfterReadTimeout(t *testing.T) {
	const readTimeout = 200 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select { // the latency: headers come well after one ReadTimeout
		case <-time.After(2 * readTimeout):
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		f.Flush()
		tick := time.NewTicker(readTimeout / 4) // heartbeats well inside ReadTimeout
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				if _, err := w.Write([]byte(":\n\n")); err != nil {
					return
				}
				f.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}))
	defer srv.Close()

	var connected atomic.Bool
	c := newSSEClient(sseClientConfig{
		URL:           srv.URL + "/sse",
		APIKey:        "k",
		OnEnvelope:    func(*ConfigEnvelope) {},
		OnStateChange: func(up bool) { connected.Store(up) },
		ReadTimeout:   readTimeout,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      20 * time.Millisecond,
	})
	c.Start()
	defer c.Stop()

	if !pollUntil(3*time.Second, connected.Load) {
		t.Fatalf("never connected: headers arriving %v after the request (> ReadTimeout %v) must not time out while the stream heartbeats every %v", 2*readTimeout, readTimeout, readTimeout/4)
	}
}
