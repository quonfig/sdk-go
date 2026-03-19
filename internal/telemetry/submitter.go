package telemetry

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	queueCapacity = 10000
	maxRetries    = 5
	sdkVersion    = "go-0.1.0"
)

// queueItem is either an EvalMatch or a ContextData.
type queueItem interface{}

// Submitter collects telemetry events and periodically submits them.
type Submitter struct {
	evalAggregator    *EvalSummaryAggregator
	shapeAggregator   *ContextShapeAggregator
	exampleAggregator *ExampleContextAggregator

	instanceHash string
	apiKey       string
	url          string
	interval     time.Duration
	httpClient   *http.Client

	queue  chan queueItem
	stopCh chan struct{}
}

// Config holds the configuration for a Submitter.
type Config struct {
	APIKey                     string
	TelemetryURL               string
	SyncInterval               time.Duration
	CollectEvaluationSummaries bool
	ContextTelemetryMode       string // "", "shapes", "periodic_example"
	InstanceHash               string
	HTTPClient                 *http.Client
}

// NewSubmitter creates a new telemetry submitter.
func NewSubmitter(cfg Config) *Submitter {
	s := &Submitter{
		instanceHash: cfg.InstanceHash,
		apiKey:       cfg.APIKey,
		url:          cfg.TelemetryURL,
		interval:     cfg.SyncInterval,
		httpClient:   cfg.HTTPClient,
		queue:        make(chan queueItem, queueCapacity),
		stopCh:       make(chan struct{}),
	}

	if s.httpClient == nil {
		s.httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	if cfg.CollectEvaluationSummaries {
		s.evalAggregator = NewEvalSummaryAggregator()
	}

	switch cfg.ContextTelemetryMode {
	case "periodic_example":
		s.shapeAggregator = NewContextShapeAggregator()
		s.exampleAggregator = NewExampleContextAggregator()
	case "shapes":
		s.shapeAggregator = NewContextShapeAggregator()
	}

	return s
}

// Start begins the queue consumer and periodic submission loop.
func (s *Submitter) Start() {
	go s.consumeQueue()
	go s.periodicSubmit()
}

// Stop flushes remaining data and stops the submitter.
func (s *Submitter) Stop() {
	close(s.stopCh)
	// Drain the queue
	s.drainQueue()
	// Final submission
	s.submit()
}

// RecordEvaluation enqueues an evaluation result for aggregation.
func (s *Submitter) RecordEvaluation(match EvalMatch) {
	if s.evalAggregator == nil || !match.isValid() {
		return
	}
	select {
	case s.queue <- match:
	default:
		// Queue full, drop
	}
}

func (m EvalMatch) isValid() bool {
	// Skip log level evaluations (same as old sdk-go)
	return m.ConfigType != "log_level_v2"
}

// RecordContext enqueues a context for shape and example aggregation.
func (s *Submitter) RecordContext(ctx ContextData) {
	if s.shapeAggregator == nil && s.exampleAggregator == nil {
		return
	}
	if len(ctx.Contexts) == 0 {
		return
	}
	select {
	case s.queue <- ctx:
	default:
		// Queue full, drop
	}
}

func (s *Submitter) consumeQueue() {
	for {
		select {
		case item := <-s.queue:
			s.processItem(item)
		case <-s.stopCh:
			return
		}
	}
}

func (s *Submitter) processItem(item queueItem) {
	switch v := item.(type) {
	case EvalMatch:
		if s.evalAggregator != nil {
			s.evalAggregator.Record(v)
		}
	case ContextData:
		if s.shapeAggregator != nil {
			s.shapeAggregator.Record(v)
		}
		if s.exampleAggregator != nil {
			s.exampleAggregator.Record(v)
		}
	}
}

func (s *Submitter) periodicSubmit() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.submit()
		case <-s.stopCh:
			return
		}
	}
}

func (s *Submitter) drainQueue() {
	for {
		select {
		case item := <-s.queue:
			s.processItem(item)
		default:
			return
		}
	}
}

func (s *Submitter) submit() {
	payload := TelemetryEvents{
		InstanceHash: s.instanceHash,
	}

	if s.evalAggregator != nil {
		if event := s.evalAggregator.GetAndClear(); event != nil {
			payload.Events = append(payload.Events, *event)
		}
	}
	if s.shapeAggregator != nil {
		if event := s.shapeAggregator.GetAndClear(); event != nil {
			payload.Events = append(payload.Events, *event)
		}
	}
	if s.exampleAggregator != nil {
		if event := s.exampleAggregator.GetAndClear(); event != nil {
			payload.Events = append(payload.Events, *event)
		}
	}

	if len(payload.Events) == 0 {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	url := fmt.Sprintf("%s/api/v1/telemetry/", s.url)
	s.postWithRetry(url, data)
}

func (s *Submitter) postWithRetry(url string, data []byte) {
	backoff := 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(data))
		if err != nil {
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Quonfig-SDK-Version", sdkVersion)

		encodedAuth := base64.StdEncoding.EncodeToString([]byte("authuser:" + s.apiKey))
		req.Header.Set("Authorization", "Basic "+encodedAuth)

		resp, err := s.httpClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}

		if attempt < maxRetries {
			time.Sleep(backoff)
			backoff *= 2
		}
	}
}
