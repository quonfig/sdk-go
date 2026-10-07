package quonfig

import (
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// qfg-goi1.2.4: SSE connect hardening.

// Item 3: a peer that accepts the TCP connection but never sends response
// headers must not wedge the stream. The inactivity watchdog is only armed
// after Do returns, so before the fix the client sat in Do forever and never
// reconnected. ResponseHeaderTimeout = ReadTimeout bounds it.
func TestSSEClientReconnectsWhenPeerNeverSendsHeaders(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
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
		URL:          "http://" + ln.Addr().String() + "/sse",
		APIKey:       "k",
		OnEnvelope:   func(*ConfigEnvelope) {},
		ReadTimeout:  200 * time.Millisecond,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     20 * time.Millisecond,
	})
	c.Start()
	defer c.Stop()

	if !pollUntil(2*time.Second, func() bool { return accepts.Load() >= 2 }) {
		t.Fatalf("accepts = %d after 2s, want >= 2: the client never gave up on a peer that sends no headers", accepts.Load())
	}
}

func TestSSEClientDefaultTransportHeaderTimeoutIsReadTimeout(t *testing.T) {
	c := newSSEClient(sseClientConfig{URL: "http://x", OnEnvelope: func(*ConfigEnvelope) {}})
	tr := c.cfg.Client.Transport.(*http.Transport)
	if tr.ResponseHeaderTimeout != 90*time.Second {
		t.Fatalf("ResponseHeaderTimeout = %v, want 90s (= default ReadTimeout; api-delivery may hold headers until the 30s heartbeat)", tr.ResponseHeaderTimeout)
	}
}
