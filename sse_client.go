package quonfig

// SSE client for real-time config updates.
//
// Ported from ReforgeHQ/sdk-go/internal/sse/sseclient.go but rewritten to use
// only the Go stdlib. Reforge depends on github.com/r3labs/sse/v2; adding a
// new external dep requires human approval per the project constitution, and
// the wire format we consume (plain JSON envelopes, no base64, no proto, no
// named events) is trivial enough that a ~100-line stdlib parser is clearer
// than a library wrapper.
//
// Event format served by api-delivery/internal/serve/sse.go (see qfg-cb3):
//
//	id: <workspace version>
//	data: <ConfigEnvelope JSON>
//
//	: keepalive       <-- SSE comment, every 30s, must be ignored
//
// Auth mirrors runtime_transport.fetchFromURL: HTTP Basic with user="1",
// password=APIKey, plus X-Quonfig-SDK-Version and Accept: text/event-stream.
//
// Reconnect policy: exponential backoff (InitialDelay → MaxDelay) with jitter,
// reset on successful event. The background loop lives until Stop().

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// sseClientConfig carries the knobs the SSE client needs. Public fields are
// unexported-package so production callers cannot accidentally muck with
// reconnect policy — NewClient wires them through.
type sseClientConfig struct {
	URL       string       // fully-formed URL, e.g. https://stream.primary.quonfig.com/api/v2/sse/config
	APIKey    string       // used as the password half of HTTP Basic "1":apiKey
	UserAgent string       // value for X-Quonfig-SDK-Version; e.g. "go-0.0.8"
	Client    *http.Client // optional; a sensible long-timeout default is used if nil

	// OnEnvelope is invoked with the parsed envelope on every successful event.
	// Required.
	OnEnvelope func(*ConfigEnvelope)

	// OnStateChange, if non-nil, is invoked with true when a connection is
	// established (after HTTP 200 response headers) and false when it drops
	// (for any reason, including Stop). Never called twice in a row with the
	// same value.
	OnStateChange func(connected bool)

	// Reconnect backoff. Zero values get sane defaults.
	InitialDelay time.Duration // default: 500ms
	MaxDelay     time.Duration // default: 30s
}

// sseClient runs a long-lived goroutine that keeps an SSE connection open,
// parses events, and invokes OnEnvelope with each parsed ConfigEnvelope.
type sseClient struct {
	cfg        sseClientConfig
	ctx        context.Context
	cancel     context.CancelFunc
	done       chan struct{}
	startOnce  sync.Once
	stopOnce   sync.Once
	connected  bool
	connectedMu sync.Mutex
}

func newSSEClient(cfg sseClientConfig) *sseClient {
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 500 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.Client == nil {
		// No read timeout — an SSE stream is long-lived by design. We only set
		// a reasonable dial/TLS handshake bound via the default transport;
		// cancellation comes from ctx.
		cfg.Client = &http.Client{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &sseClient{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

// Start kicks off the background reconnect loop. Idempotent.
func (c *sseClient) Start() {
	c.startOnce.Do(func() {
		go c.runLoop()
	})
}

// Stop cancels the in-flight connection, drains the loop, and waits for the
// goroutine to exit. Idempotent and safe to call before Start.
func (c *sseClient) Stop() {
	c.stopOnce.Do(func() {
		c.cancel()
	})
	// Only wait if Start was actually called.
	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
		// Belt-and-suspenders: if the goroutine is wedged for some reason
		// (shouldn't happen — cancel closes the body), don't deadlock Close.
	}
}

func (c *sseClient) runLoop() {
	defer close(c.done)
	defer c.setConnected(false) // guarantee one final "disconnected" signal

	delay := c.cfg.InitialDelay
	for {
		// Connection attempt.
		connectedOK := c.connectOnce()
		if c.ctx.Err() != nil {
			return
		}

		if connectedOK {
			// We had a live stream that then ended — reset the backoff so the
			// next retry is snappy. A server-initiated close is normal (the
			// LB recycles connections periodically); don't punish it.
			delay = c.cfg.InitialDelay
		}

		// Jittered sleep before reconnecting.
		jitter := time.Duration(rand.Int63n(int64(delay) + 1))
		sleep := delay/2 + jitter/2
		t := time.NewTimer(sleep)
		select {
		case <-c.ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}

		// Exponential backoff (on failed connect specifically).
		if !connectedOK {
			delay *= 2
			if delay > c.cfg.MaxDelay {
				delay = c.cfg.MaxDelay
			}
		}
	}
}

// connectOnce opens a single SSE request and reads until the body errors or
// context is cancelled. Returns true iff response headers made it back 200 OK
// (i.e. the connection was "live" at some point), false if we never got that
// far. Callers use the return value to distinguish backoff-worthy failures
// (DNS, refused, 401) from normal session recycling.
func (c *sseClient) connectOnce() bool {
	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, c.cfg.URL, nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth("1", c.cfg.APIKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if c.cfg.UserAgent != "" {
		req.Header.Set("X-Quonfig-SDK-Version", c.cfg.UserAgent)
	}

	resp, err := c.cfg.Client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Drain a small amount of body so the connection can be reused, then
		// give up. We intentionally treat 401/403 the same as a network
		// failure — a customer rotating their key should eventually be
		// picked up by the next reconnect.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return false
	}

	c.setConnected(true)
	defer c.setConnected(false)

	c.parseStream(resp.Body)
	return true
}

// parseStream reads SSE frames from r and calls OnEnvelope for each complete
// event. It follows a minimal subset of the SSE spec sufficient for our
// server's format:
//
//   - Lines starting with "data:" accumulate a per-event buffer.
//   - Lines starting with ":" are comments (keepalives) — ignored.
//   - Lines starting with "id:" are ignored here (server uses it for version
//     tracking; we don't need last-event-id reconnect semantics yet).
//   - An empty line terminates the event: the accumulated data is fed to
//     OnEnvelope as a ConfigEnvelope.
//
// This deliberately does NOT handle multi-line data frames by concatenating
// with newlines — api-delivery always emits single-line JSON. If that
// changes, the bufio.Scanner pre-sized line buffer and this function both
// need an update.
func (c *sseClient) parseStream(r io.Reader) {
	scanner := bufio.NewScanner(r)
	// Envelopes can be large (500 flags × ~500B rules, plus meta). Allow up to
	// 4 MiB lines which is well over what we've observed in production.
	const maxLine = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 64*1024), maxLine)

	var dataBuf bytes.Buffer
	flush := func() {
		if dataBuf.Len() == 0 {
			return
		}
		var env ConfigEnvelope
		if err := json.Unmarshal(dataBuf.Bytes(), &env); err == nil {
			if c.cfg.OnEnvelope != nil {
				c.cfg.OnEnvelope(&env)
			}
		}
		// else: malformed payload — swallow so a single bad event doesn't
		// tear down the stream. The HTTP poller is a safety net.
		dataBuf.Reset()
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			flush()
			continue
		}
		if line[0] == ':' {
			// Comment (keepalive) — ignore.
			continue
		}
		// Accept both "data: <x>" and "data:<x>" (optional single space).
		if rest, ok := stripFieldPrefix(line, "data:"); ok {
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(rest)
			continue
		}
		// Other SSE fields ("id:", "event:", "retry:") currently unused.
	}
	// Stream ended (EOF or read error). Any pending event without a trailing
	// blank line is discarded — matches real SSE server behavior.
	_ = scanner.Err()
}

// stripFieldPrefix returns (value, true) if s starts with prefix (optionally
// followed by a single space). The SSE spec allows either "field:value" or
// "field: value"; real servers tend to emit the latter.
func stripFieldPrefix(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || s[:len(prefix)] != prefix {
		return "", false
	}
	rest := s[len(prefix):]
	if len(rest) > 0 && rest[0] == ' ' {
		rest = rest[1:]
	}
	return rest, true
}

// setConnected records connection state transitions and fires the state
// callback exactly once per actual edge.
func (c *sseClient) setConnected(v bool) {
	c.connectedMu.Lock()
	changed := c.connected != v
	c.connected = v
	cb := c.cfg.OnStateChange
	c.connectedMu.Unlock()
	if changed && cb != nil {
		// Run in a goroutine so a slow callback can't stall the reader. The
		// callback itself is expected to be cheap (metric update), but we
		// don't want to pin that contract on every caller.
		go cb(v)
	}
}
