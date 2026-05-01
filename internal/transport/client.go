package transport

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	quonfig "github.com/quonfig/sdk-go"
	"github.com/quonfig/sdk-go/internal/version"
)

// Client fetches configs from the Quonfig API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	etag       string
}

// New creates a new transport Client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		httpClient: &http.Client{},
		baseURL:    baseURL,
		apiKey:     apiKey,
	}
}

// FetchResult holds the outcome of a config fetch.
type FetchResult struct {
	Envelope   *quonfig.ConfigEnvelope
	NotChanged bool
}

// FetchConfigs fetches configs from GET /api/v2/configs.
// It uses ETag-based caching: sends If-None-Match on subsequent requests
// and returns NotChanged=true on 304.
func (c *Client) FetchConfigs() (*FetchResult, error) {
	url := c.baseURL + "/api/v2/configs"

	req, err := http.NewRequest("GET", url, nil)
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
		return nil, fmt.Errorf("fetching configs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return &FetchResult{NotChanged: true}, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		c.etag = etag
	}

	var envelope quonfig.ConfigEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &FetchResult{Envelope: &envelope}, nil
}
