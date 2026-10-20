package quonfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Startup-failure behavior (qfg-9p4i.7). The ITD init-failure cases
// (internal/fixtures/get_or_raise_generated_test.go) pin the NewClient
// contract; these cover what the corpus cannot: readiness, heal, wait,
// the logging contract and the deprecated aliases.

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newBufLogger(level slog.Level) (*slog.Logger, *syncBuffer) {
	b := &syncBuffer{}
	return slog.New(slog.NewTextHandler(b, &slog.HandlerOptions{Level: level})), b
}

// switchableDelivery answers 503 until up is set, then serves one config.
type switchableDelivery struct {
	up       atomic.Bool
	requests atomic.Int64
	srv      *httptest.Server
}

func newSwitchableDelivery(t *testing.T) *switchableDelivery {
	t.Helper()
	d := &switchableDelivery{}
	d.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		d.requests.Add(1)
		if !d.up.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ConfigEnvelope{
			Configs: []ConfigResponse{{
				Key:       "app.name",
				ValueType: ValueTypeString,
				Default:   RuleSet{Rules: []Rule{{Value: Value{Type: ValueTypeString, Value: "live"}}}},
			}},
			Meta: Meta{Version: "v-live", Environment: "Production", Generation: 3},
		})
	}))
	t.Cleanup(d.srv.Close)
	return d
}

func shrinkInitCadence(t *testing.T) {
	t.Helper()
	ob, om := initBackoffBase, initBackoffMax
	op, ow, obg := initProgressEvery, initWaitProgressEvery, initBackgroundEvery
	initBackoffBase, initBackoffMax = 20*time.Millisecond, 50*time.Millisecond
	initProgressEvery, initWaitProgressEvery, initBackgroundEvery = 100*time.Millisecond, 100*time.Millisecond, 100*time.Millisecond
	t.Cleanup(func() {
		initBackoffBase, initBackoffMax = ob, om
		initProgressEvery, initWaitProgressEvery, initBackgroundEvery = op, ow, obg
	})
}

func baseInitOpts(urls []string, logger *slog.Logger) []Option {
	return []Option{
		WithSdkKey("secret-sdk-key-123"),
		WithAPIURLs(urls),
		WithSSE(false),
		WithFallbackPoll(false, 0),
		WithAllTelemetryDisabled(),
		WithQuonfigUserContext(false),
		WithLogger(logger),
	}
}

// The repro from the 2026-10-16 audit (M2/L4): with both legs refusing,
// NewClient used to return a live-looking client and log nothing at WARN+.
func TestInitFailureDefaultFailsAndLogsAtWarnPlus(t *testing.T) {
	logger, logs := newBufLogger(slog.LevelWarn)
	start := time.Now()
	c, err := NewClient(append(baseInitOpts([]string{"http://127.0.0.1:1", "http://127.0.0.1:2"}, logger),
		WithInitTimeout(600*time.Millisecond))...)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Nil(t, c)
	var ie *InitError
	require.ErrorAs(t, err, &ie)
	assert.Equal(t, InitErrorUnavailable, ie.Reason)
	assert.NotErrorIs(t, err, ErrInitializationTimeout)
	assert.GreaterOrEqual(t, ie.Attempts, 2, "fail retries until the timeout")
	assert.GreaterOrEqual(t, elapsed, 550*time.Millisecond, "fail waits out initTimeout on refused legs")
	assert.Less(t, elapsed, 3*time.Second)

	out := logs.String()
	assert.Contains(t, out, "level=WARN msg=\"[quonfig] init: config not loaded yet, retrying until initTimeout (600ms). primary=127.0.0.1:1: ")
	assert.Contains(t, out, "secondary=127.0.0.1:2: ")
	assert.Contains(t, out, "level=ERROR msg=\"[quonfig] init FAILED: no config after 0.6s.")
	assert.Contains(t, out, "onInitFailure=fail")
	assert.Contains(t, out, "event=quonfig.init.failed")
	assert.NotContains(t, out, "secret-sdk-key-123", "the SDK key is never logged")
	assert.NotContains(t, err.Error(), "secret-sdk-key-123")
}

func TestInitFailureHangIsTimeoutReason(t *testing.T) {
	logger, _ := newBufLogger(slog.LevelError)
	hang := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	c, err := NewClient(append(baseInitOpts([]string{"https://p.example.test", "https://s.example.test"}, logger),
		WithHTTPClient(hang), WithInitTimeout(150*time.Millisecond))...)
	require.Error(t, err)
	assert.Nil(t, c)
	assert.ErrorIs(t, err, ErrInitializationTimeout, "a hang keeps the old errors.Is contract")
	var ie *InitError
	require.ErrorAs(t, err, &ie)
	assert.Equal(t, InitErrorTimeout, ie.Reason)
	assert.ErrorIs(t, err, context.DeadlineExceeded, "Unwrap gives the cause")
}

func TestInitFailureUnauthorizedFailsAtOnce(t *testing.T) {
	logger, logs := newBufLogger(slog.LevelWarn)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	start := time.Now()
	c, err := NewClient(append(baseInitOpts([]string{srv.URL, srv.URL}, logger), WithInitTimeout(5*time.Second))...)
	require.Error(t, err)
	assert.Nil(t, c)
	assert.Less(t, time.Since(start), time.Second)
	var ie *InitError
	require.ErrorAs(t, err, &ie)
	assert.Equal(t, InitErrorUnauthorized, ie.Reason)
	assert.Contains(t, logs.String(), "[quonfig] init FAILED: SDK key rejected (401) by primary and secondary.")
}

func TestInitFailureContinueReturnsThenHeals(t *testing.T) {
	shrinkInitCadence(t)
	d := newSwitchableDelivery(t)
	logger, logs := newBufLogger(slog.LevelWarn)
	c, err := NewClient(append(baseInitOpts([]string{d.srv.URL, d.srv.URL}, logger),
		WithInitTimeout(150*time.Millisecond), WithOnInitFailure(InitFailureContinue))...)
	require.NoError(t, err)
	require.NotNil(t, c)
	defer c.Close()

	assert.False(t, c.Ready())
	_, ok, gerr := c.GetStringValue("app.name", nil)
	assert.False(t, ok)
	assert.ErrorIs(t, gerr, ErrNotFound, "continue answers as an empty store")
	det := c.EvaluateDetails("app.name", nil)
	assert.Equal(t, ErrorCodeProviderNotReady, det.ErrorCode)
	assert.Contains(t, logs.String(), "level=ERROR msg=\"[quonfig] init: no config after 0.")
	assert.Contains(t, logs.String(), "onInitFailure=continue: STARTING ON CODE DEFAULTS.")

	// No SSE, no poller: the init retry loop alone must heal the client.
	d.up.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, c.WaitForReady(ctx))
	assert.True(t, c.Ready())
	v, ok, gerr := c.GetStringValue("app.name", nil)
	require.NoError(t, gerr)
	assert.True(t, ok)
	assert.Equal(t, "live", v)
	assert.Eventually(t, func() bool {
		return strings.Contains(logs.String(), "level=WARN msg=\"[quonfig] init recovered: config loaded after ") &&
			strings.Contains(logs.String(), "(version v-live). Now serving live config.")
	}, 2*time.Second, 10*time.Millisecond)
}

func TestInitFailureContinueBackgroundWarns(t *testing.T) {
	shrinkInitCadence(t)
	d := newSwitchableDelivery(t)
	logger, logs := newBufLogger(slog.LevelWarn)
	c, err := NewClient(append(baseInitOpts([]string{d.srv.URL}, logger),
		WithInitTimeout(50*time.Millisecond), WithOnInitFailure(InitFailureContinue))...)
	require.NoError(t, err)
	defer c.Close()
	assert.Eventually(t, func() bool {
		return strings.Contains(logs.String(), "[quonfig] still not connected to Quonfig delivery (")
	}, 2*time.Second, 10*time.Millisecond)
	assert.Contains(t, logs.String(), "serving defaults. primary=")
}

func TestInitFailureWaitBlocksUntilDeliveryReturns(t *testing.T) {
	shrinkInitCadence(t)
	d := newSwitchableDelivery(t)
	logger, logs := newBufLogger(slog.LevelWarn)
	go func() {
		time.Sleep(400 * time.Millisecond)
		d.up.Store(true)
	}()
	start := time.Now()
	c, err := NewClient(append(baseInitOpts([]string{d.srv.URL, d.srv.URL}, logger),
		WithInitTimeout(100*time.Millisecond), WithOnInitFailure(InitFailureWait))...)
	require.NoError(t, err)
	defer c.Close()
	assert.GreaterOrEqual(t, time.Since(start), 350*time.Millisecond, "wait blocks past initTimeout")
	assert.True(t, c.Ready())
	v, ok, gerr := c.GetStringValue("app.name", nil)
	require.NoError(t, gerr)
	assert.True(t, ok)
	assert.Equal(t, "live", v)
	assert.Contains(t, logs.String(), "onInitFailure=wait: still blocking startup and retrying indefinitely.")
}

func TestInitSuccessIsReadyAndQuietAtWarn(t *testing.T) {
	d := newSwitchableDelivery(t)
	d.up.Store(true)
	logger, logs := newBufLogger(slog.LevelWarn)
	c, err := NewClient(baseInitOpts([]string{d.srv.URL, d.srv.URL}, logger)...)
	require.NoError(t, err)
	defer c.Close()
	assert.True(t, c.Ready())
	require.NoError(t, c.WaitForReady(context.Background()))
	assert.Empty(t, logs.String(), "a healthy boot never logs above DEBUG")
	assert.Equal(t, int64(1), d.requests.Load())
}

func TestInitFailureDeprecatedAliasWarnsOnce(t *testing.T) {
	deprecatedInitAliasWarn.Delete(ReturnZeroValue)
	defer deprecatedInitAliasWarn.Delete(ReturnZeroValue)
	logger, logs := newBufLogger(slog.LevelWarn)
	for i := 0; i < 2; i++ {
		c, err := NewClient(append(baseInitOpts([]string{"http://127.0.0.1:1"}, logger),
			WithInitTimeout(10*time.Millisecond), WithOnInitFailure(ReturnZeroValue))...) //nolint:staticcheck // alias under test
		require.NoError(t, err, "ReturnZeroValue is an alias of continue")
		c.Close()
	}
	assert.Equal(t, 1, strings.Count(logs.String(), "[quonfig] WithOnInitFailure(quonfig.ReturnZeroValue) is deprecated; use quonfig.InitFailureContinue"))
}

func TestWithOnInitFailureRejectsUnknownValue(t *testing.T) {
	_, err := NewClient(WithOnInitFailure(OnInitFailure(42)))
	require.Error(t, err)
	assert.Equal(t, "fail", InitFailureFail.String())
	assert.Equal(t, "continue", ReturnZeroValue.String())
	assert.Equal(t, "fail", ReturnError.String())
	assert.Equal(t, "wait", InitFailureWait.String())
}

func TestWaitForReadyHonorsContextAndClose(t *testing.T) {
	logger, _ := newBufLogger(slog.LevelError)
	c, err := NewClient(append(baseInitOpts([]string{"http://127.0.0.1:1"}, logger),
		WithInitTimeout(10*time.Millisecond), WithOnInitFailure(InitFailureContinue))...)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, c.WaitForReady(ctx), context.DeadlineExceeded)
	c.Close()
	assert.True(t, errors.Is(c.WaitForReady(context.Background()), errClientClosed))
}
