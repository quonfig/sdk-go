package quonfig

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// lockedBuf is a goroutine-safe log sink: background fetches log while the
// test reads.
type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// qfg-9dxb.6 (audit HIGH 2): one config whose value fails to decode used to
// reject the whole envelope on every install path, so one bad key blocked
// init or froze the workspace. Now the bad entry is skipped with a WARN
// naming the key and the rest installs.

const goodFlagJSON = `{"key":"good.flag","type":"feature_flag","valueType":"bool","default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"bool","value":true}}]}}`

// A json config stored in the legacy stringified form: Value.UnmarshalJSON
// rejects it.
const badJSONConfigJSON = `{"key":"bad.json","type":"config","valueType":"json","default":{"rules":[{"criteria":[{"operator":"ALWAYS_TRUE"}],"value":{"type":"json","value":"{\"a\":1}"}}]}}`

func envelopeJSON(configs ...string) string {
	return `{"configs":[` + strings.Join(configs, ",") + `],"meta":{"version":"gen-1","environment":"Production","generation":1}}`
}

func TestBadConfigEntrySkippedOnHTTPInit(t *testing.T) {
	body := envelopeJSON(goodFlagJSON, badJSONConfigJSON)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	var logBuf lockedBuf
	client, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(false),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
		WithLogger(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(client.Close)
	// Let background workers start before Close runs in Cleanup: closing
	// mid-startup trips a known, separate init/Close race (qfg-9dxb.6
	// medium, not fixed here).
	waitFor(t, 3*time.Second, client.FallbackPollerActive, "background workers never started")

	v, ok, err := client.GetBoolValue("good.flag", nil)
	if err != nil || !ok || !v {
		t.Fatalf("good.flag = %v ok=%v err=%v; want true, true, nil", v, ok, err)
	}
	if _, ok, _ := client.GetJSONValue("bad.json", nil); ok {
		t.Errorf("bad.json should have been skipped, but it was found")
	}
	if !strings.Contains(logBuf.String(), "bad.json") {
		t.Errorf("expected a WARN naming bad.json, got log: %q", logBuf.String())
	}
}

func TestBadConfigEntrySkippedOnSSE(t *testing.T) {
	var got []*ConfigEnvelope
	c := newSSEClient(sseClientConfig{
		OnEnvelope: func(env *ConfigEnvelope) { got = append(got, env) },
	})
	stream := "data: " + envelopeJSON(goodFlagJSON, badJSONConfigJSON) + "\n\n"
	_ = c.parseStream(strings.NewReader(stream))
	if len(got) != 1 {
		t.Fatalf("OnEnvelope called %d times, want 1", len(got))
	}
	if n := len(got[0].Configs); n != 1 || got[0].Configs[0].Key != "good.flag" {
		t.Fatalf("delivered configs = %+v, want only good.flag", got[0].Configs)
	}
}

// If every entry fails to decode, the envelope is rejected as before rather
// than installed as an empty workspace (which would wipe every key).
func TestAllConfigEntriesBadRejectsEnvelope(t *testing.T) {
	var got []*ConfigEnvelope
	c := newSSEClient(sseClientConfig{
		OnEnvelope: func(env *ConfigEnvelope) { got = append(got, env) },
	})
	stream := "data: " + envelopeJSON(badJSONConfigJSON) + "\n\n"
	_ = c.parseStream(strings.NewReader(stream))
	if len(got) != 0 {
		t.Fatalf("OnEnvelope called %d times with an all-bad envelope, want 0", len(got))
	}
}
