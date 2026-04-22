package quonfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(status int, headers map[string]string, body interface{}) *http.Response {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}

	httpHeaders := make(http.Header, len(headers))
	for key, value := range headers {
		httpHeaders.Set(key, value)
	}

	return &http.Response{
		StatusCode: status,
		Header:     httpHeaders,
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
}

func TestNewClientInitializesAndUsesEnvLookup(t *testing.T) {
	var requests atomic.Int32
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			requests.Add(1)
			return jsonResponse(http.StatusOK, map[string]string{"Content-Type": "application/json"}, ConfigEnvelope{
				Configs: []ConfigResponse{
					{
						Key:       "provided.a.number",
						ValueType: ValueTypeInt,
						Default: RuleSet{
							Rules: []Rule{
								{
									Value: Value{
										Type: ValueTypeProvided,
										Value: &ProvidedData{
											Source: "ENV_VAR",
											Lookup: "IS_A_NUMBER",
										},
									},
								},
							},
						},
					},
				},
				Meta: Meta{
					Version:     "v1",
					Environment: "Production",
				},
			}), nil
		}),
	}

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithHTTPClient(httpClient),
		WithEnvLookup(func(key string) (string, bool) {
			if key == "IS_A_NUMBER" {
				return "1234", true
			}
			return "", false
		}),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	value, ok, err := client.GetIntValue("provided.a.number", nil)
	if err != nil {
		t.Fatalf("GetIntValue returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected provided.a.number to resolve")
	}
	if value != 1234 {
		t.Fatalf("expected 1234, got %d", value)
	}
	if requests.Load() != 1 {
		t.Fatalf("expected exactly one initial fetch, got %d", requests.Load())
	}
}

func TestNewClientInitTimeoutHonorsFailurePolicy(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			select {
			case <-time.After(150 * time.Millisecond):
			case <-r.Context().Done():
				return nil, r.Context().Err()
			}

			return jsonResponse(http.StatusOK, map[string]string{"Content-Type": "application/json"}, ConfigEnvelope{
				Configs: []ConfigResponse{
					{
						Key:       "app.name",
						ValueType: ValueTypeString,
						Default: RuleSet{
							Rules: []Rule{{Value: Value{Type: ValueTypeString, Value: "demo"}}},
						},
					},
				},
				Meta: Meta{
					Version:     "v1",
					Environment: "Production",
				},
			}), nil
		}),
	}

	errorClient, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithHTTPClient(httpClient),
		WithInitTimeout(25*time.Millisecond),
		WithOnInitFailure(ReturnError),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	_, _, err = errorClient.GetStringValue("app.name", nil)
	if !errors.Is(err, ErrInitializationTimeout) {
		t.Fatalf("expected ErrInitializationTimeout, got %v", err)
	}

	zeroClient, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithHTTPClient(httpClient),
		WithInitTimeout(25*time.Millisecond),
		WithOnInitFailure(ReturnZeroValue),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	value, ok, err := zeroClient.GetStringValue("app.name", nil)
	if err != nil {
		t.Fatalf("expected no error for ReturnZeroValue, got %v", err)
	}
	if ok || value != "" {
		t.Fatalf("expected zero-value result, got value=%q ok=%v", value, ok)
	}
}

func TestClientRefreshUsesETagAndUpdatesStore(t *testing.T) {
	var revision atomic.Int32
	revision.Store(1)

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			current := revision.Load()
			etag := "v1"
			value := "one"
			if current == 2 {
				etag = "v2"
				value = "two"
			}

			if r.Header.Get("If-None-Match") == etag {
				return jsonResponse(http.StatusNotModified, nil, nil), nil
			}

			return jsonResponse(http.StatusOK, map[string]string{
				"ETag":         etag,
				"Content-Type": "application/json",
			}, ConfigEnvelope{
				Configs: []ConfigResponse{
					{
						Key:       "app.name",
						ValueType: ValueTypeString,
						Default: RuleSet{
							Rules: []Rule{{Value: Value{Type: ValueTypeString, Value: value}}},
						},
					},
				},
				Meta: Meta{
					Version:     etag,
					Environment: "Production",
				},
			}), nil
		}),
	}

	client, err := NewClient(
		WithAPIKey("test-key"),
		WithAPIURLs([]string{"https://example.test"}),
		WithHTTPClient(httpClient),
	)
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	value, ok, err := client.GetStringValue("app.name", nil)
	if err != nil || !ok || value != "one" {
		t.Fatalf("expected initial value one, got value=%q ok=%v err=%v", value, ok, err)
	}

	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}

	value, ok, err = client.GetStringValue("app.name", nil)
	if err != nil || !ok || value != "one" {
		t.Fatalf("expected unchanged value one after 304 refresh, got value=%q ok=%v err=%v", value, ok, err)
	}

	revision.Store(2)
	if err := client.Refresh(); err != nil {
		t.Fatalf("Refresh returned error after version bump: %v", err)
	}

	value, ok, err = client.GetStringValue("app.name", nil)
	if err != nil || !ok || value != "two" {
		t.Fatalf("expected refreshed value two, got value=%q ok=%v err=%v", value, ok, err)
	}
}
