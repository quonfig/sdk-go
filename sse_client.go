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
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"runtime/debug"
	"sync"
	"sync/atomic"
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

	// ReadTimeout bounds how long the SSE socket may sit idle before the
	// client closes it and reconnects. Defaults to 90s = 3x the 30s server
	// heartbeat (one missed heartbeat as noise tolerance, two missed as a
	// clear signal). Set explicitly to a small value in tests; 0 means use
	// the default. There is no "disable" — silent stalls are the failure
	// mode that motivated this knob, and disabling it brings back the bug.
	ReadTimeout time.Duration

	// Logger is used to record recovered panics from the OnEnvelope callback
	// and other unexpected internal errors. Nil falls back to slog.Default().
	Logger *slog.Logger
}

// sseClient runs a long-lived goroutine that keeps an SSE connection open,
// parses events, and invokes OnEnvelope with each parsed ConfigEnvelope.
type sseClient struct {
	cfg         sseClientConfig
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	startOnce   sync.Once
	stopOnce    sync.Once
	connected   bool
	connectedMu sync.Mutex

	// workerRestarts tracks the quonfig_sdk_worker_restart_total counter,
	// labeled by reason (e.g. "callback_panic"). Layer 1 is implied — the SSE
	// stream itself. The map is allocated lazily under restartsMu.
	restartsMu     sync.Mutex
	workerRestarts map[string]*atomic.Int64
}

func newSSEClient(cfg sseClientConfig) *sseClient {
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 500 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 90 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Client == nil {
		// Force HTTP/1.1 on the SSE socket. http.DefaultTransport defaults to
		// negotiating HTTP/2 over TLS; an h2 stream can stall (silent at the
		// stream layer) while connection-level frames keep flowing — toxiproxy
		// is TCP-only and cannot reproduce that, so we'd ship the bug
		// undetected. Clone DefaultTransport (keeps the sensible dial/TLS
		// timeouts) and disable h2 negotiation.
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.ForceAttemptHTTP2 = false
		tr.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		cfg.Client = &http.Client{Transport: tr}
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
	// Per-attempt context so the read-deadline timer can cancel just this
	// request without affecting the long-lived client context.
	reqCtx, cancelReq := context.WithCancel(c.ctx)
	defer cancelReq()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.cfg.URL, nil)
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
	defer func() { _ = resp.Body.Close() }()

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

	// Wrap the body so each successful read resets a watchdog timer; if no
	// bytes arrive within ReadTimeout the timer cancels reqCtx, the next
	// body read errors, parseStream returns, and runLoop reconnects.
	timer := time.AfterFunc(c.cfg.ReadTimeout, cancelReq)
	defer timer.Stop()
	c.parseStream(&deadlineResetReader{r: resp.Body, timer: timer, d: c.cfg.ReadTimeout})
	return true
}

// deadlineResetReader wraps an io.Reader to enforce a per-read inactivity
// deadline via a time.AfterFunc-backed watchdog. Each successful Read resets
// the timer; if the timer fires the parent context is cancelled by the
// caller, and subsequent reads error out.
type deadlineResetReader struct {
	r     io.Reader
	timer *time.Timer
	d     time.Duration
}

func (d *deadlineResetReader) Read(p []byte) (int, error) {
	n, err := d.r.Read(p)
	if n > 0 {
		d.timer.Reset(d.d)
	}
	return n, err
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
				c.invokeOnEnvelope(&env)
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

// invokeOnEnvelope calls the user-supplied OnEnvelope callback under a
// defer/recover so a panic in customer code cannot kill the SSE goroutine
// (which would silently freeze the SDK at whatever envelope was current at
// the time of the crash). On recovery we log at ERROR with the panic value
// and stack, bump the quonfig_sdk_worker_restart_total{layer="1",
// reason="callback_panic"} counter, and return — the parseStream loop then
// continues with the next event.
func (c *sseClient) invokeOnEnvelope(env *ConfigEnvelope) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		c.incWorkerRestart("callback_panic")
		c.cfg.Logger.Error("quonfig: OnEnvelope callback panicked; SSE loop continuing",
			slog.Any("panic", r),
			slog.String("stack", string(debug.Stack())),
			slog.String("layer", "1"),
			slog.String("reason", "callback_panic"),
		)
	}()
	c.cfg.OnEnvelope(env)
}

// workerRestartTotal returns the current value of the
// quonfig_sdk_worker_restart_total{layer="1",reason=<reason>} counter. The
// counter is incremented on recovered panics inside OnEnvelope (reason =
// "callback_panic"); future causes (e.g. reader-loop panic) reuse this same
// surface with a different reason label.
func (c *sseClient) workerRestartTotal(reason string) int64 {
	c.restartsMu.Lock()
	v := c.workerRestarts[reason]
	c.restartsMu.Unlock()
	if v == nil {
		return 0
	}
	return v.Load()
}

// incWorkerRestart bumps the counter for a given reason label, allocating the
// map entry on first use.
func (c *sseClient) incWorkerRestart(reason string) {
	c.restartsMu.Lock()
	v := c.workerRestarts[reason]
	if v == nil {
		if c.workerRestarts == nil {
			c.workerRestarts = make(map[string]*atomic.Int64)
		}
		v = &atomic.Int64{}
		c.workerRestarts[reason] = v
	}
	c.restartsMu.Unlock()
	v.Add(1)
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
