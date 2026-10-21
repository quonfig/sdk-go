package quonfig

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Init-failure conformance fixes (qfg-9p4i.17.1).

// withUserinfo returns srvURL with user:pass@ spliced in after the scheme.
func withUserinfo(srvURL, userinfo string) string {
	return strings.Replace(srvURL, "://", "://"+userinfo+"@", 1)
}

// D4: neither the init error, its wrapped cause, nor the init log lines may
// carry URL userinfo or the upstream response body.
func TestInitErrorRedactsUserinfoAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("UPSTREAM-SECRET-BODY"))
	}))
	defer srv.Close()

	logger, logs := newBufLogger(slog.LevelDebug)
	c, err := NewClient(append(baseInitOpts([]string{withUserinfo(srv.URL, "alice:hunter2")}, logger),
		WithInitTimeout(150*time.Millisecond))...)
	require.Error(t, err)
	assert.Nil(t, c)
	var ie *InitError
	require.ErrorAs(t, err, &ie)
	assert.Equal(t, InitErrorUnavailable, ie.Reason)

	for name, s := range map[string]string{
		"Error()":  err.Error(),
		"Unwrap()": errors.Unwrap(err).Error(),
		"logs":     logs.String(),
	} {
		assert.NotContains(t, s, "hunter2", "%s leaks the URL password", name)
		assert.NotContains(t, s, "alice", "%s leaks the URL username", name)
		assert.NotContains(t, s, "UPSTREAM-SECRET-BODY", "%s leaks the upstream body", name)
	}
	assert.Contains(t, errors.Unwrap(err).Error(), "503", "the status is kept")
}

// D4: a transport error (refused) built from a URL with userinfo is redacted
// at the source, including the net/http *url.Error it wraps.
func TestInitTransportErrorRedactsUserinfo(t *testing.T) {
	logger, logs := newBufLogger(slog.LevelDebug)
	c, err := NewClient(append(baseInitOpts([]string{"http://alice:hunter2@127.0.0.1:1"}, logger),
		WithInitTimeout(100*time.Millisecond))...)
	require.Error(t, err)
	assert.Nil(t, c)
	for name, s := range map[string]string{
		"Error()":  err.Error(),
		"Unwrap()": errors.Unwrap(err).Error(),
		"logs":     logs.String(),
	} {
		assert.NotContains(t, s, "hunter2", "%s leaks the URL password", name)
		assert.NotContains(t, s, "alice", "%s leaks the URL username", name)
	}
	assert.Contains(t, errors.Unwrap(err).Error(), "127.0.0.1:1", "the host is kept")
}

// A redacted *url.Error still reports Timeout(), so a hang keeps the
// "timeout" reason (D2).
func TestInitHangWithUserinfoIsStillTimeout(t *testing.T) {
	logger, _ := newBufLogger(slog.LevelError)
	hang := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	_, err := NewClient(append(baseInitOpts([]string{"https://alice:hunter2@p.example.test"}, logger),
		WithHTTPClient(hang), WithInitTimeout(150*time.Millisecond))...)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInitializationTimeout)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.NotContains(t, errors.Unwrap(err).Error(), "hunter2")
}

// The unauthorized line names the legs that are actually configured.
func TestInitUnauthorizedSingleURLWording(t *testing.T) {
	logger, logs := newBufLogger(slog.LevelWarn)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	_, err := NewClient(append(baseInitOpts([]string{srv.URL}, logger), WithInitTimeout(5*time.Second))...)
	require.Error(t, err)
	out := logs.String()
	assert.Contains(t, out, "[quonfig] init FAILED: SDK key rejected (401) by primary. Check the key")
	assert.NotContains(t, out, "rejected (401) by primary and secondary")
}

// Under wait, the auth-rejected ERROR fires the first time an attempt is
// all-auth, even when attempt 1 was not (it was a 503).
func TestInitWaitAuthErrorFiresOnLaterAttempt(t *testing.T) {
	shrinkInitCadence(t)
	var n atomic.Int64
	var up atomic.Bool
	d := newSwitchableDelivery(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case up.Load():
			d.srv.Config.Handler.ServeHTTP(w, r)
		case n.Add(1) == 1:
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer srv.Close()
	d.up.Store(true)
	go func() {
		time.Sleep(300 * time.Millisecond)
		up.Store(true)
	}()

	logger, logs := newBufLogger(slog.LevelWarn)
	c, err := NewClient(append(baseInitOpts([]string{srv.URL}, logger),
		WithInitTimeout(5*time.Second), WithOnInitFailure(InitFailureWait))...)
	require.NoError(t, err)
	defer c.Close()
	out := logs.String()
	assert.Contains(t, out, "level=ERROR msg=\"[quonfig] init: SDK key rejected (401) by primary. Check the key and environment. onInitFailure=wait")
	assert.Equal(t, 1, strings.Count(out, "event=quonfig.init.unauthorized"), "logged once")
}
