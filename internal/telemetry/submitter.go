package telemetry

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quonfig/sdk-go/internal/version"
)

const queueCapacity = 10000

// errAborted is the cancel cause Stop() uses for an in-flight tick POST.
var errAborted = errors.New("telemetry POST aborted by shutdown")

// queueItem is either an EvalMatch or a ContextData.
type queueItem interface{}

// Submitter collects telemetry events and sends them under the transport
// policy (qfg-y8je.6): one tick per SyncInterval (60s), one POST in flight,
// a 15s deadline per POST on the request context, failed batches retained
// byte-for-byte (5 batches / 2MB / 5 min) and resent no sooner than 30s after
// a failure or after Retry-After (max 10 min), disable on 401/403/404, and
// logging only on data loss and state change.
type Submitter struct {
	evalAggregator     *EvalSummaryAggregator
	shapeAggregator    *ContextShapeAggregator
	exampleAggregator  *ExampleContextAggregator
	failoverAggregator *FailoverAggregator

	cfg                Config // resolved
	instanceHash       string
	apiKey             string
	url                string
	httpClient         *http.Client
	ownsHTTPClient     bool
	logger             *slog.Logger
	clock              Clock
	tq                 *transportQueue
	runCtx             context.Context
	abortRun           context.CancelCauseFunc
	pending            atomic.Int64
	procMu             sync.Mutex // serializes aggregation between the consumer and a tick
	queue              chan queueItem
	stopCh             chan struct{}
	consumerDone       chan struct{}
	startOnce          sync.Once
	stopOnce           sync.Once
	consumerStartedFlg atomic.Bool

	mu          sync.Mutex
	started     bool
	closed      bool
	timer       Timer
	tickRunning bool
	ticks       sync.WaitGroup
	postCtx     context.Context // the in-flight POST's context, nil when idle
}

// Config holds the configuration for a Submitter. Zero values fall back to
// the shipped defaults.
type Config struct {
	APIKey                     string
	TelemetryURL               string
	SyncInterval               time.Duration
	CollectEvaluationSummaries bool
	ContextTelemetryMode       string // "", "shapes_only", "periodic_example" ("shapes" still accepted as a deprecated alias for "shapes_only")
	InstanceHash               string
	// HTTPClient, when set, is used for the POSTs. The Timeout deadline is
	// applied on the request context regardless, so a client without a
	// timeout cannot remove it; ConnectTimeout applies only to the default
	// client.
	HTTPClient *http.Client
	Logger     *slog.Logger

	Timeout                time.Duration
	ConnectTimeout         time.Duration
	MaxRetainedBatches     int
	MaxRetainedBytes       int
	MaxRetainedAge         time.Duration
	MaxEvaluationSummaries int
	MaxContextShapeFields  int
	MaxExampleContexts     int

	// Clock is a test seam; nil means the wall clock.
	Clock Clock
}

func durOr(v, def time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return def
}

func intOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func (c Config) resolved() Config {
	c.SyncInterval = durOr(c.SyncInterval, DefaultSyncInterval)
	c.Timeout = durOr(c.Timeout, DefaultTimeout)
	c.ConnectTimeout = durOr(c.ConnectTimeout, DefaultConnectTimeout)
	c.MaxRetainedBatches = intOr(c.MaxRetainedBatches, DefaultMaxRetainedBatches)
	c.MaxRetainedBytes = intOr(c.MaxRetainedBytes, DefaultMaxRetainedBytes)
	c.MaxRetainedAge = durOr(c.MaxRetainedAge, DefaultMaxRetainedAge)
	c.MaxEvaluationSummaries = intOr(c.MaxEvaluationSummaries, DefaultMaxEvaluationSummaries)
	c.MaxContextShapeFields = intOr(c.MaxContextShapeFields, DefaultMaxContextShapeFields)
	c.MaxExampleContexts = intOr(c.MaxExampleContexts, DefaultMaxExampleContexts)
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	if c.Clock == nil {
		c.Clock = RealClock
	}
	return c
}

// newDefaultHTTPClient: a dedicated client with the P1 connect + TLS
// deadline. Idle connections close after 30s, well under Fly's 60s idle
// close, so a 60s tick never races a reused socket.
func newDefaultHTTPClient(connectTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   connectTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: connectTimeout,
			IdleConnTimeout:     30 * time.Second,
			MaxIdleConns:        2,
		},
	}
}

// NewSubmitter creates a new telemetry submitter.
func NewSubmitter(cfg Config) *Submitter {
	cfg = cfg.resolved()
	s := &Submitter{
		cfg:          cfg,
		instanceHash: cfg.InstanceHash,
		apiKey:       cfg.APIKey,
		url:          fmt.Sprintf("%s/api/v1/telemetry/", cfg.TelemetryURL),
		httpClient:   cfg.HTTPClient,
		logger:       cfg.Logger,
		clock:        cfg.Clock,
		queue:        make(chan queueItem, queueCapacity),
		stopCh:       make(chan struct{}),
		consumerDone: make(chan struct{}),
	}
	if s.httpClient == nil {
		s.httpClient = newDefaultHTTPClient(cfg.ConnectTimeout)
		s.ownsHTTPClient = true
	}
	s.runCtx, s.abortRun = context.WithCancelCause(context.Background())
	s.tq = &transportQueue{
		logger:             s.logger,
		clock:              s.clock,
		telemetryURL:       s.url,
		maxRetainedBatches: cfg.MaxRetainedBatches,
		maxRetainedBytes:   cfg.MaxRetainedBytes,
		maxRetainedAge:     cfg.MaxRetainedAge,
	}

	// Failover counters carry no user data and are the operational signal for
	// the secondary-delivery hardening, so they ride any enabled telemetry
	// stream regardless of the eval/context opt-outs. The Submitter itself is
	// only constructed when telemetry is enabled, so this still honors a full
	// telemetry opt-out.
	s.failoverAggregator = NewFailoverAggregator()

	if cfg.CollectEvaluationSummaries {
		s.evalAggregator = NewEvalSummaryAggregatorWithCap(cfg.MaxEvaluationSummaries)
	}

	switch cfg.ContextTelemetryMode {
	case "periodic_example":
		s.shapeAggregator = NewContextShapeAggregatorWithCap(cfg.MaxContextShapeFields)
		s.exampleAggregator = NewExampleContextAggregatorWithCap(cfg.MaxExampleContexts)
	case "shapes_only", "shapes":
		// "shapes" is the pre-1.0 wire value, kept as a deprecated alias
		// for one minor cycle while consumers migrate to "shapes_only".
		s.shapeAggregator = NewContextShapeAggregatorWithCap(cfg.MaxContextShapeFields)
	}

	return s
}

// Start begins the queue consumer and the tick timer.
func (s *Submitter) Start() {
	s.startOnce.Do(func() {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		s.started = true
		s.scheduleLocked()
		s.mu.Unlock()
		s.consumerStartedFlg.Store(true)
		go s.consumeQueue()
	})
}

// scheduleLocked arms the next tick. Fixed cadence: tick k fires at
// k * SyncInterval regardless of how long a drain takes. Caller holds mu.
func (s *Submitter) scheduleLocked() {
	s.timer = s.clock.AfterFunc(s.cfg.SyncInterval, func() {
		s.mu.Lock()
		s.timer = nil
		if s.closed || s.tq.isDisabled() {
			s.mu.Unlock()
			return
		}
		s.scheduleLocked()
		s.mu.Unlock()
		s.Tick()
	})
}

// Stop is the P8 shutdown: stop the timer, abort any in-flight POST, then
// give the live window one POST with a 5s deadline. The retained queue is
// not drained. Idempotent; bounded by the 5s deadline.
func (s *Submitter) Stop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		if s.timer != nil {
			s.timer.Stop()
			s.timer = nil
		}
		s.mu.Unlock()

		s.abortRun(errAborted)
		s.ticks.Wait()

		close(s.stopCh)
		if s.consumerStartedFlg.Load() {
			<-s.consumerDone
		}

		if !s.tq.isDisabled() {
			if body := s.serializeWindow(); body != nil {
				deadline := ShutdownFlushDeadline
				if s.cfg.Timeout < deadline {
					deadline = s.cfg.Timeout
				}
				s.sendFinal(body, deadline)
			}
		}
		if s.ownsHTTPClient {
			s.httpClient.CloseIdleConnections()
		}
	})
}

// Tick runs one tick of the contract's model: skip if closed, disabled or a
// POST is in flight (P2; the live window keeps aggregating); expire aged
// batches; skip if the 30s floor or Retry-After has not elapsed; serialize
// the live window once and append it; drain oldest-first, stopping at the
// first failure. Safe to call from any goroutine; returns at once when busy.
func (s *Submitter) Tick() {
	s.mu.Lock()
	if s.closed || s.tickRunning || s.tq.isDisabled() {
		s.mu.Unlock()
		return
	}
	s.tickRunning = true
	s.ticks.Add(1)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.tickRunning = false
		s.mu.Unlock()
		s.ticks.Done()
	}()

	s.tq.expire()
	if !s.tq.sendAllowed() {
		return
	}
	if body := s.serializeWindow(); body != nil {
		s.tq.append(body)
	}
	s.drain()
}

func (s *Submitter) drain() {
	defer s.tq.dropOversize()
	for {
		batch := s.tq.head()
		if batch == nil {
			return
		}
		res, err := s.post(s.runCtx, batch.body, s.cfg.Timeout)
		if err != nil {
			if errors.Is(err, errAborted) {
				return // shutdown: the batch dies with the process
			}
			s.tq.onRetryableFailure(batch, describeError(err), "")
			return
		}
		switch classifyStatus(res.status) {
		case statusOK:
			s.tq.onSuccess(batch)
		case statusRetryable:
			s.tq.onRetryableFailure(batch, fmt.Sprint(res.status), res.retryAfter)
			return
		case statusAuth:
			s.tq.disable(res.status)
			s.onDisabled()
			return
		default:
			s.tq.onRejected(batch, res.status, res.bodySnippet)
		}
	}
}

// onDisabled stops the timer and stops aggregating for a dead endpoint.
func (s *Submitter) onDisabled() {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.mu.Unlock()
	s.serializeWindow() // discard what was collected
}

// sendFinal: one POST bounded by deadline. Never retains, never touches the
// outage episode.
func (s *Submitter) sendFinal(body []byte, deadline time.Duration) {
	res, err := s.post(context.Background(), body, deadline)
	result := ""
	if err != nil {
		result = describeError(err)
	} else if classifyStatus(res.status) != statusOK {
		result = fmt.Sprint(res.status)
	}
	if result != "" {
		count, _ := s.tq.counts()
		s.logger.Debug(fmt.Sprintf(
			"quonfig: Telemetry final flush at shutdown failed (%s); %d bytes dropped, %d retained batch(es) abandoned",
			result, len(body), count))
	}
}

type httpResult struct {
	status      int
	retryAfter  string
	bodySnippet string
}

type requestError struct {
	reason string // "timeout", "network"
	err    error
}

func (e *requestError) Error() string { return e.reason + ": " + e.err.Error() }
func (e *requestError) Unwrap() error { return e.err }

func describeError(err error) string {
	var re *requestError
	if errors.As(err, &re) {
		if re.reason == "timeout" {
			return "timeout"
		}
		return "network error: " + re.err.Error()
	}
	return "network error: " + err.Error()
}

// post sends one telemetry POST. The policy deadline rides the request
// context (on the injected clock), so a caller-supplied http.Client without a
// timeout cannot remove it. Returns errAborted (wrapped) if parent was
// canceled by Stop().
func (s *Submitter) post(parent context.Context, body []byte, timeout time.Duration) (httpResult, error) {
	ctx, cancel := s.clock.WithTimeout(parent, timeout)
	defer cancel()

	s.mu.Lock()
	s.postCtx = ctx
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.postCtx = nil
		s.mu.Unlock()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return httpResult{}, &requestError{reason: "network", err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Quonfig-SDK-Version", version.Header())
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("1:"+s.apiKey)))

	resp, err := s.httpClient.Do(req)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		snippet, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr == nil {
			_, readErr = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		}
		if readErr == nil || ctx.Err() == nil {
			return httpResult{
				status:      resp.StatusCode,
				retryAfter:  resp.Header.Get("Retry-After"),
				bodySnippet: string(snippet),
			}, nil
		}
		err = readErr
	}
	if context.Cause(parent) == errAborted {
		return httpResult{}, errAborted
	}
	if errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
		return httpResult{}, &requestError{reason: "timeout", err: err}
	}
	return httpResult{}, &requestError{reason: "network", err: err}
}

// RecordEvaluation enqueues an evaluation result for aggregation.
func (s *Submitter) RecordEvaluation(match EvalMatch) {
	if s.evalAggregator == nil || !match.isValid() || s.tq.isDisabled() {
		return
	}
	s.enqueue(match)
}

func (s *Submitter) enqueue(item queueItem) {
	s.pending.Add(1)
	select {
	case s.queue <- item:
	default:
		s.pending.Add(-1) // queue full, drop
	}
}

func (m EvalMatch) isValid() bool {
	// Skip log level evaluations from telemetry
	return m.ConfigType != "log_level"
}

// RecordHedgeFired records one config-fetch cycle whose hedge fired the
// secondary leg. Safe to call before Start and on a zero Submitter.
func (s *Submitter) RecordHedgeFired() {
	if s.failoverAggregator != nil {
		s.failoverAggregator.RecordHedgeFired()
	}
}

// RecordGuardRejected records one install dropped by the reject-older guard
// because the payload was strictly older than the held generation (qfg-rr5b).
func (s *Submitter) RecordGuardRejected() {
	if s.failoverAggregator != nil {
		s.failoverAggregator.RecordGuardRejected()
	}
}

// RecordResolvedFrom records one successful HTTP install by the leg that served
// it (sourceIndex 0 = primary, > 0 = secondary; a negative index is ignored).
func (s *Submitter) RecordResolvedFrom(sourceIndex int) {
	if s.failoverAggregator != nil {
		s.failoverAggregator.RecordResolvedFrom(sourceIndex)
	}
}

// RecordContext enqueues a context for shape and example aggregation.
func (s *Submitter) RecordContext(ctx ContextData) {
	if s.shapeAggregator == nil && s.exampleAggregator == nil {
		return
	}
	if len(ctx.Contexts) == 0 || s.tq.isDisabled() {
		return
	}
	s.enqueue(ctx)
}

func (s *Submitter) consumeQueue() {
	defer close(s.consumerDone)
	for {
		select {
		case item := <-s.queue:
			s.procMu.Lock()
			s.processItem(item)
			s.procMu.Unlock()
			s.pending.Add(-1)
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

// drainQueue moves every queued record into the aggregators.
func (s *Submitter) drainQueue() {
	s.procMu.Lock()
	defer s.procMu.Unlock()
	s.drainQueueLocked()
}

// drainQueueLocked moves every queued record into the aggregators. Caller
// holds procMu.
func (s *Submitter) drainQueueLocked() {
	for {
		select {
		case item := <-s.queue:
			s.processItem(item)
			s.pending.Add(-1)
		default:
			return
		}
	}
}

// serializeWindow drains the aggregators into one serialized payload, or nil
// when the window is empty. This is the only serialization: the retained
// queue stores and resends these exact bytes (P5, P9).
func (s *Submitter) serializeWindow() []byte {
	s.procMu.Lock()
	defer s.procMu.Unlock()
	s.drainQueueLocked()

	payload := TelemetryEvents{InstanceHash: s.instanceHash}
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
	if s.failoverAggregator != nil {
		if event := s.failoverAggregator.GetAndClear(); event != nil {
			payload.Events = append(payload.Events, *event)
		}
	}
	if len(payload.Events) == 0 {
		return nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		s.logger.Debug("quonfig: Telemetry batch could not be serialized: " + err.Error())
		return nil
	}
	return data
}

// DebugState is test-visible transport state (the contract's retained_count,
// retained_bytes and telemetry_enabled, plus liveness of background work).
type DebugState struct {
	RetainedCount   int
	RetainedBytes   int
	Enabled         bool
	TickRunning     bool
	PostInFlight    bool
	PostCanceled    bool // the in-flight POST's context is already done
	TimerActive     bool
	ConsumerRunning bool
}

// DebugState reports the transport state. Internal; for tests.
func (s *Submitter) DebugState() DebugState {
	count, bytes := s.tq.counts()
	st := DebugState{RetainedCount: count, RetainedBytes: bytes, Enabled: !s.tq.isDisabled()}
	s.mu.Lock()
	st.TickRunning = s.tickRunning
	st.TimerActive = s.timer != nil
	if s.postCtx != nil {
		st.PostInFlight = true
		st.PostCanceled = s.postCtx.Err() != nil
	}
	s.mu.Unlock()
	if s.consumerStartedFlg.Load() {
		select {
		case <-s.consumerDone:
		default:
			st.ConsumerRunning = true
		}
	}
	return st
}

// PendingRecords is the number of recorded evaluations/contexts not yet
// aggregated. Internal; for tests.
func (s *Submitter) PendingRecords() int64 { return s.pending.Load() }

// ResolvedConfig returns the configuration with defaults applied.
func (s *Submitter) ResolvedConfig() Config { return s.cfg }

// HTTPClient returns the client used for telemetry POSTs.
func (s *Submitter) HTTPClient() *http.Client { return s.httpClient }
