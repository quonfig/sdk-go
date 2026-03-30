package quonfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const sdkVersion = "0.1.0"

type fetchResult struct {
	Envelope   *ConfigEnvelope
	NotChanged bool
}

type runtimeTransport struct {
	httpClient *http.Client
	baseURLs   []string
	apiKey     string
	etag       string
}

func newRuntimeTransport(baseURLs []string, apiKey string, httpClient *http.Client) *runtimeTransport {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	trimmed := make([]string, len(baseURLs))
	for i, u := range baseURLs {
		trimmed[i] = strings.TrimRight(u, "/")
	}
	return &runtimeTransport{
		httpClient: httpClient,
		baseURLs:   trimmed,
		apiKey:     apiKey,
	}
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
