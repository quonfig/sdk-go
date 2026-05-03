package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSubmitter_SubmitsEvalSummaries(t *testing.T) {
	var mu sync.Mutex
	var received []TelemetryEvents

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload TelemetryEvents
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("failed to unmarshal: %v", err)
			w.WriteHeader(500)
			return
		}
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()

	s := NewSubmitter(Config{
		APIKey:                     "test-key",
		TelemetryURL:               server.URL,
		SyncInterval:               50 * time.Millisecond,
		CollectEvaluationSummaries: true,
		ContextTelemetryMode:       "periodic_example",
		InstanceHash:               "test-instance",
	})

	s.Start()

	// Record some evaluations
	for i := 0; i < 10; i++ {
		s.RecordEvaluation(EvalMatch{
			ConfigID:      "cfg-1",
			ConfigKey:     "feature.flag",
			ConfigType:    "feature_flag",
			SelectedValue: true,
			Reason:        2,
		})
	}

	// Record a context
	s.RecordContext(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {"key": "user-123", "email": "alice@test.com"},
		},
	})

	// Wait for periodic submission
	time.Sleep(200 * time.Millisecond)

	s.Stop()

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("expected at least one submission")
	}

	// Check that the submission had the right instance hash
	if received[0].InstanceHash != "test-instance" {
		t.Errorf("expected instance hash 'test-instance', got %q", received[0].InstanceHash)
	}

	// Verify events were included
	totalEvents := 0
	for _, payload := range received {
		totalEvents += len(payload.Events)
	}
	if totalEvents == 0 {
		t.Error("expected events in submission")
	}
}

func TestSubmitter_SkipsLogLevelEvaluations(t *testing.T) {
	s := NewSubmitter(Config{
		CollectEvaluationSummaries: true,
		ContextTelemetryMode:       "",
		SyncInterval:               time.Minute,
		InstanceHash:               "test",
	})

	s.RecordEvaluation(EvalMatch{
		ConfigID:      "cfg-log",
		ConfigKey:     "log.level.myservice",
		ConfigType:    "log_level",
		SelectedValue: "DEBUG",
	})

	// Process the queue
	s.drainQueue()

	event := s.evalAggregator.GetAndClear()
	if event != nil {
		t.Error("expected log_level evaluations to be skipped")
	}
}

func TestSubmitter_DisabledTelemetry(t *testing.T) {
	s := NewSubmitter(Config{
		CollectEvaluationSummaries: false,
		ContextTelemetryMode:       "",
		SyncInterval:               time.Minute,
		InstanceHash:               "test",
	})

	// Should not panic with nil aggregators
	s.RecordEvaluation(EvalMatch{
		ConfigID:      "cfg-1",
		ConfigKey:     "flag",
		ConfigType:    "feature_flag",
		SelectedValue: true,
	})
	s.RecordContext(ContextData{
		Contexts: map[string]map[string]interface{}{
			"user": {"key": "u1"},
		},
	})

	if s.evalAggregator != nil {
		t.Error("expected nil eval aggregator when disabled")
	}
	if s.shapeAggregator != nil {
		t.Error("expected nil shape aggregator when disabled")
	}
	if s.exampleAggregator != nil {
		t.Error("expected nil example aggregator when disabled")
	}
}

func TestSubmitter_ContextTelemetryShapesOnly(t *testing.T) {
	s := NewSubmitter(Config{
		CollectEvaluationSummaries: false,
		ContextTelemetryMode:       "shapes",
		SyncInterval:               time.Minute,
		InstanceHash:               "test",
	})

	if s.shapeAggregator == nil {
		t.Error("expected shape aggregator when mode is 'shapes'")
	}
	if s.exampleAggregator != nil {
		t.Error("expected nil example aggregator when mode is 'shapes'")
	}
}

func TestSubmitter_StopFlushesData(t *testing.T) {
	var mu sync.Mutex
	var received []TelemetryEvents

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload TelemetryEvents
		_ = json.Unmarshal(body, &payload)
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer server.Close()

	s := NewSubmitter(Config{
		APIKey:                     "test-key",
		TelemetryURL:               server.URL,
		SyncInterval:               10 * time.Minute, // Long interval so periodic doesn't fire
		CollectEvaluationSummaries: true,
		InstanceHash:               "test-instance",
	})

	s.Start()

	s.RecordEvaluation(EvalMatch{
		ConfigID:      "cfg-1",
		ConfigKey:     "flag",
		ConfigType:    "feature_flag",
		SelectedValue: true,
	})

	// Give queue consumer time to process
	time.Sleep(50 * time.Millisecond)

	// Stop should flush
	s.Stop()

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("expected Stop() to flush pending data")
	}
}

func TestSubmitter_AuthHeader(t *testing.T) {
	var authHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer server.Close()

	s := NewSubmitter(Config{
		APIKey:                     "my-sdk-key",
		TelemetryURL:               server.URL,
		SyncInterval:               time.Minute,
		CollectEvaluationSummaries: true,
		InstanceHash:               "test",
	})

	// Directly record and submit
	s.evalAggregator.Record(EvalMatch{
		ConfigID:      "cfg-1",
		ConfigKey:     "flag",
		ConfigType:    "feature_flag",
		SelectedValue: true,
	})
	s.submit()

	if authHeader == "" {
		t.Fatal("expected Authorization header")
	}
	// Should be Basic auth with base64("1:my-sdk-key")
	expected := "Basic MTpteS1zZGsta2V5"
	if authHeader != expected {
		t.Errorf("expected auth header %q, got %q", expected, authHeader)
	}
}
