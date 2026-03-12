package quonfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const sdkVersion = "0.1.0"

type fetchResult struct {
	Envelope   *ConfigEnvelope
	NotChanged bool
}

type runtimeTransport struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	etag       string
}

func newRuntimeTransport(baseURL, apiKey string, httpClient *http.Client) *runtimeTransport {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return &runtimeTransport{
		httpClient: httpClient,
		baseURL:    baseURL,
		apiKey:     apiKey,
	}
}

func (c *runtimeTransport) FetchConfigs(ctx context.Context) (*fetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v2/configs", nil)
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
		return nil, fmt.Errorf("fetching configs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &fetchResult{NotChanged: true}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
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
