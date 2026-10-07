package quonfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quonfig/sdk-go/internal/version"
)

type fetchResult struct {
	Envelope   *ConfigEnvelope
	NotChanged bool
	// SourceIndex is the index into baseURLs of the leg that produced this
	// result. 0 is the primary leg, 1 the secondary, etc. Used so the SDK can
	// report which upstream a config was resolved from (failover observability).
	SourceIndex int
	// prevETag and storedETag let the caller undo this response's ETag write
	// when the guard drops a gen<=0 payload (see rollbackETag).
	prevETag   string
	storedETag string
}

type runtimeTransport struct {
	httpClient *http.Client
	baseURLs   []string
	// streamURLs is parallel to baseURLs: streamURLs[i] is the SSE URL derived
	// from baseURLs[i] by prepending "stream." to the hostname. If
	// testStreamURLOverride was set, every entry in streamURLs is that single
	// override value — this is a test-only escape hatch because an
	// httptest.NewServer cannot provide a stream.* hostname.
	streamURLs []string
	apiKey     string
	// etags is PER-LEG: etags[i] is the last ETag seen from baseURLs[i].
	// The hedge runs both legs concurrently, so a single shared ETag would let a
	// 304 from one leg mask the other (and would be a data race). etagMu guards
	// reads/writes; each leg snapshots its slot before the request and writes the
	// response ETag back after — the network wait happens with no lock held.
	etagMu sync.Mutex
	etags  []string
	// hedgeDelay is how long the hedge waits for the primary leg before ALSO
	// firing the secondary in parallel (it does not cancel the primary). A primary
	// that succeeds or errors before this fires the secondary immediately on error
	// and never on a fast success. Zero means DefaultConfigFetchHedgeDelay.
	hedgeDelay time.Duration
	// hedgeAbort is the per-leg hard deadline on the hedged path. It must exceed
	// the longest healable primary latency so a late-but-newer primary heals
	// forward instead of aborting, and must be < InitTimeout so the init-path heal
	// leg is not clipped. Zero means DefaultConfigFetchHedgeAbort.
	hedgeAbort time.Duration
	// logger receives the WARN for a config entry skipped during decode.
	// Nil means slog.Default(). Set once at construction.
	logger *slog.Logger
}

// legResult carries one hedged leg's outcome to the caller. Exactly one
// legResult is emitted per fired leg; Res.SourceIndex identifies the leg.
type legResult struct {
	Res *fetchResult
	Err error
}

// DefaultConfigFetchTimeout bounds one per-URL config-fetch attempt on the
// SEQUENTIAL FetchConfigs path when no explicit WithConfigFetchTimeout is
// supplied. ~3s is short enough that a hung primary fails over to the secondary
// well inside a default 10s InitTimeout, yet long enough to tolerate a
// slow-but-healthy upstream. This is a per-attempt deadline only — it does NOT
// touch the long-lived SSE stream, which keeps its own 120s disconnect
// threshold. The hedged config-fetch path uses hedgeDelay/hedgeAbort instead.
const DefaultConfigFetchTimeout = 3 * time.Second

// DefaultConfigFetchHedgeDelay is how long the hedge waits for the primary leg
// before ALSO firing the secondary in parallel. ~2s is below a realistic
// slow-but-alive primary's worst case yet far enough below the per-leg abort
// that a healthy sub-second primary is NEVER hedged (the secondary stays a cold
// standby and a healthy system adds zero secondary load). Tunable via
// WithConfigFetchHedgeDelay.
const DefaultConfigFetchHedgeDelay = 2 * time.Second

// DefaultConfigFetchHedgeAbort is the per-leg hard-abort deadline on the hedged
// path. It MUST exceed the longest healable primary latency so a late-but-newer
// primary heals forward (rather than aborting), and MUST be < InitTimeout so the
// init-path heal leg is not clipped. Tunable via WithConfigFetchHedgeAbort.
const DefaultConfigFetchHedgeAbort = 6 * time.Second

func newRuntimeTransport(baseURLs []string, apiKey string, httpClient *http.Client) *runtimeTransport {
	return newRuntimeTransportWithStreamOverride(baseURLs, apiKey, httpClient, "")
}

// newRuntimeTransportWithStreamOverride is the internal constructor. If
// streamOverride is non-empty it is used verbatim for every SSE URL, bypassing
// stream.* derivation. Tests pass Options.testStreamURLOverride through here.
func newRuntimeTransportWithStreamOverride(baseURLs []string, apiKey string, httpClient *http.Client, streamOverride string) *runtimeTransport {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	trimmed := make([]string, len(baseURLs))
	streamURLs := make([]string, len(baseURLs))
	for i, u := range baseURLs {
		t := strings.TrimRight(u, "/")
		trimmed[i] = t
		if streamOverride != "" {
			streamURLs[i] = strings.TrimRight(streamOverride, "/")
			continue
		}
		// Best-effort derivation. If a caller passes garbage it will surface
		// later when the SSE client actually tries to dial — we don't want to
		// block HTTP polling over a bad stream derivation, so fall back to the
		// base URL unmodified if derive fails.
		if s, err := deriveStreamURL(t); err == nil {
			streamURLs[i] = s
		} else {
			streamURLs[i] = t
		}
	}
	return &runtimeTransport{
		httpClient: httpClient,
		baseURLs:   trimmed,
		streamURLs: streamURLs,
		apiKey:     apiKey,
		etags:      make([]string, len(trimmed)),
	}
}

// streamURLFor returns the SSE URL (with the /api/v2/sse/config path appended)
// corresponding to baseURLs[i]. Used by the SSE client when it opens the
// long-lived connection.
func (c *runtimeTransport) streamURLFor(i int) string {
	if i < 0 || i >= len(c.streamURLs) {
		return ""
	}
	return c.streamURLs[i] + "/api/v2/sse/config"
}

// FetchConfigsHedged fires the primary leg (index 0) and, if it has not settled
// within hedgeDelay OR errors fast, ALSO fires the secondary leg (index 1) in
// parallel — without cancelling the primary. Both legs run under their own
// hedgeAbort deadline and their own ETag slot. Each fired leg emits exactly one
// legResult on the returned channel in arrival order; the channel is closed once
// every fired leg has settled, so the number drained equals the number fired.
// The caller installs each successful result through the reject-older guard so
// watermark-max (higher generation wins; late older does not regress; late newer
// heals forward) falls out without any source ranking. A fast healthy primary
// means the secondary is never contacted (cold standby).
func (c *runtimeTransport) FetchConfigsHedged(ctx context.Context, hedgeDelay, hedgeAbort time.Duration) <-chan legResult {
	if hedgeDelay <= 0 {
		hedgeDelay = DefaultConfigFetchHedgeDelay
	}
	if hedgeAbort <= 0 {
		hedgeAbort = DefaultConfigFetchHedgeAbort
	}
	out := make(chan legResult, len(c.baseURLs)+1)

	go func() {
		defer close(out)

		hasSecondary := len(c.baseURLs) > 1
		var wg sync.WaitGroup
		var secondaryFired atomic.Bool

		fireSecondary := func() {
			if !hasSecondary {
				return
			}
			if secondaryFired.CompareAndSwap(false, true) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					out <- c.fetchFromURLAt(ctx, 1, hedgeAbort)
				}()
			}
		}

		// Fire the primary. Its result is forwarded to the caller AND mirrored to a
		// private channel so the arbiter below can inspect it to decide the hedge.
		prim := make(chan legResult, 1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			lr := c.fetchFromURLAt(ctx, 0, hedgeAbort)
			prim <- lr
			out <- lr
		}()

		timer := time.NewTimer(hedgeDelay)
		defer timer.Stop()

		select {
		case lr := <-prim:
			// Primary settled before the hedge delay.
			if lr.Err != nil {
				fireSecondary() // fast error -> hedge now
			} else {
				secondaryFired.Store(true) // fast success/304 -> never hedge
			}
		case <-timer.C:
			// Timer elapsed. Re-check the primary so a primary that JUST won the
			// boundary race does not trigger an unnecessary hedge.
			select {
			case lr := <-prim:
				if lr.Err != nil {
					fireSecondary()
				}
			default:
				fireSecondary() // primary still in flight -> hedge in parallel
			}
		}

		wg.Wait()
	}()

	return out
}

// fetchFromURLAt fetches GET /api/v2/configs from baseURLs[i], using only that
// leg's ETag slot (snapshot under etagMu before the request, write-back after),
// bounded by its own abort deadline. It fully reads/decodes the body before
// returning, so cancelling on return is safe. The returned legResult.Res
// (when non-nil) carries SourceIndex=i.
// maxErrorBodyBytes caps how much of a non-200 config-fetch body is read
// into the returned error.
const maxErrorBodyBytes = 1024

func (c *runtimeTransport) fetchFromURLAt(ctx context.Context, i int, abort time.Duration) legResult {
	if i < 0 || i >= len(c.baseURLs) {
		return legResult{Err: fmt.Errorf("leg index %d out of range", i)}
	}
	baseURL := c.baseURLs[i]

	if abort > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, abort)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v2/configs", nil)
	if err != nil {
		return legResult{Err: fmt.Errorf("creating request: %w", err)}
	}

	req.SetBasicAuth("1", c.apiKey)
	req.Header.Set("X-Quonfig-SDK-Version", version.Header())
	req.Header.Set("Accept", "application/json")

	c.etagMu.Lock()
	etag := c.etags[i]
	c.etagMu.Unlock()
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return legResult{Err: fmt.Errorf("fetching configs from %s: %w", baseURL, err)}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return legResult{Res: &fetchResult{NotChanged: true, SourceIndex: i}}
	}

	if resp.StatusCode != http.StatusOK {
		// The body ends up in the error getters return and in logs, so cap
		// it: a proxy error page must not ride along in full.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return legResult{Err: fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, baseURL, strings.TrimSpace(string(body)))}
	}

	envelope, err := decodeEnvelope(json.NewDecoder(resp.Body).Decode, c.logger)
	if err != nil {
		return legResult{Err: fmt.Errorf("decoding response: %w", err)}
	}
	// A 200 that is not a config envelope is a leg error, so the hedge and
	// failover proceed exactly as for a 5xx (qfg-9dxb.3).
	if err := envelope.validate(); err != nil {
		return legResult{Err: fmt.Errorf("invalid response from %s: %w", baseURL, err)}
	}

	// Store the ETag only after the body decoded and validated. Storing it
	// first let a truncated or junk 200 pin itself through later 304s.
	newEtag := resp.Header.Get("ETag")
	if newEtag != "" {
		c.etagMu.Lock()
		c.etags[i] = newEtag
		c.etagMu.Unlock()
	}

	return legResult{Res: &fetchResult{Envelope: envelope, SourceIndex: i, prevETag: etag, storedETag: newEtag}}
}

// rollbackETag undoes the ETag write of a 200 whose gen<=0 payload the guard
// dropped (qfg-9dxb.9). Keeping it would let the server's later same-sha
// repaired-generation response 304 forever. Only rolls back if the slot still
// holds the ETag this response wrote, so a concurrent leg's write is kept.
func (c *runtimeTransport) rollbackETag(res *fetchResult) {
	if res == nil || res.storedETag == "" || res.SourceIndex < 0 || res.SourceIndex >= len(c.etags) {
		return
	}
	c.etagMu.Lock()
	defer c.etagMu.Unlock()
	if c.etags[res.SourceIndex] == res.storedETag {
		c.etags[res.SourceIndex] = res.prevETag
	}
}
