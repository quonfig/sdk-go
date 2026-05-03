package quonfig

import (
	"bytes"
	"testing"
)

// FuzzSSEParseStream feeds arbitrary byte streams into the SSE frame parser
// to surface panics, infinite loops, or buffer mishandling. The parser
// consumes data straight off the network, so a malformed stream from a
// hostile or buggy proxy must not crash the SDK.
func FuzzSSEParseStream(f *testing.F) {
	seeds := [][]byte{
		[]byte(""),
		[]byte("\n"),
		[]byte(": keepalive\n\n"),
		[]byte("data: {}\n\n"),
		[]byte("data:{\"meta\":{\"version\":\"v1\"},\"configs\":[]}\n\n"),
		[]byte("id: 1\ndata: {}\n\n"),
		[]byte("data: not-json\n\n"),
		[]byte("data: {\"configs\":[]\n\n"),
		[]byte("data: {}"), // no terminating blank line
		[]byte(": c1\n\n: c2\n\ndata: {}\n\n"),
		bytes.Repeat([]byte("data: x\n"), 50),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		c := newSSEClient(sseClientConfig{
			URL:    "http://example.test",
			APIKey: "k",
			OnEnvelope: func(*ConfigEnvelope) {
				// Receive but do nothing — we only care that parseStream is
				// safe on arbitrary input.
			},
		})
		c.parseStream(bytes.NewReader(data))
	})
}
