package quonfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	etag       string
	// fetchTimeout bounds a single per-URL config-fetch attempt. Each leg in
	// FetchConfigs gets its own context.WithTimeout(fetchTimeout) so a hung
	// upstream aborts fast and leaves budget to reach the next leg inside the
	// caller's overall deadline (e.g. InitTimeout). Zero means use
	// DefaultConfigFetchTimeout. Set once at construction; read-only thereafter.
	fetchTimeout time.Duration
}

// DefaultConfigFetchTimeout bounds one per-URL config-fetch attempt when no
// explicit WithConfigFetchTimeout is supplied. ~3s is short enough that a hung
// primary fails over to the secondary well inside a default 10s InitTimeout,
// yet long enough to tolerate a slow-but-healthy upstream. This is a
// per-attempt deadline only — it does NOT touch the long-lived SSE stream,
// which keeps its own 120s disconnect threshold.
const DefaultConfigFetchTimeout = 3 * time.Second

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

// FetchConfigs tries each base URL in order, returning the first successful result.
func (c *runtimeTransport) FetchConfigs(ctx context.Context) (*fetchResult, error) {
	var lastErr error

	for i, baseURL := range c.baseURLs {
		result, err := c.fetchFromURL(ctx, baseURL)
		if err != nil {
			lastErr = err
			continue
		}
		result.SourceIndex = i
		return result, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("all API URLs failed")
}

func (c *runtimeTransport) fetchFromURL(ctx context.Context, baseURL string) (*fetchResult, error) {
	// Bound this single attempt so a hung upstream (accepts the TCP connection
	// but never responds) aborts fast instead of blocking on the caller's
	// overall deadline. fetchFromURL fully reads/decodes the body before
	// returning, so cancelling on return is safe. context.WithTimeout takes the
	// earlier of (parent deadline, fetchTimeout), so a short InitTimeout still
	// wins; the per-URL timeout only ever shortens the wait, never extends it.
	timeout := c.fetchTimeout
	if timeout <= 0 {
		timeout = DefaultConfigFetchTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v2/configs", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.SetBasicAuth("1", c.apiKey)
	req.Header.Set("X-Quonfig-SDK-Version", version.Header())
	req.Header.Set("Accept", "application/json")
	if c.etag != "" {
		req.Header.Set("If-None-Match", c.etag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching configs from %s: %w", baseURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return &fetchResult{NotChanged: true}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, baseURL, string(body))
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		c.etag = etag
	}

	var envelope ConfigEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &fetchResult{Envelope: &envelope}, nil
}
