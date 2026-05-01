package quonfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const sdkVersion = "0.1.1"

type fetchResult struct {
	Envelope   *ConfigEnvelope
	NotChanged bool
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
}

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

	for _, baseURL := range c.baseURLs {
		result, err := c.fetchFromURL(ctx, baseURL)
		if err != nil {
			lastErr = err
			continue
		}
		return result, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("all API URLs failed")
}

func (c *runtimeTransport) fetchFromURL(ctx context.Context, baseURL string) (*fetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v2/configs", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.SetBasicAuth("1", c.apiKey)
	req.Header.Set("X-Quonfig-SDK-Version", "go-"+sdkVersion)
	req.Header.Set("Accept", "application/json")
	if c.etag != "" {
		req.Header.Set("If-None-Match", c.etag)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching configs from %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

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
