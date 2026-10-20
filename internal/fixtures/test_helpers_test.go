package fixtures

// Hand-written support for the generated *_generated_test.go files (emitted by
// integration-test-data/generators/src/targets/go.ts).
//
// Every integration case runs through the PUBLIC quonfig.Client exactly as a
// customer would: typed getters, FeatureIsOn, WithGlobalContext /
// WithContext for the context tiers, and the client's real telemetry
// reporter drained over HTTP. Nothing in this package evaluates, resolves,
// coerces or aggregates on its own (qfg-2agi.31). If a helper here starts
// re-implementing SDK logic, the integration suite stops testing the SDK.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	quonfig "github.com/quonfig/sdk-go"
)

const (
	dataDir = "../../../integration-test-data/data/integration-tests"
)

// testEnvVars is the process environment the integration data expects
// (ENV_VAR-provided configs and the decryption key). MISSING_ENV_VAR is
// intentionally NOT set. Cases that need more set their own via t.Setenv.
var testEnvVars = map[string]string{
	"PREFAB_INTEGRATION_TEST_ENCRYPTION_KEY": "c87ba22d8662282abe8a0e4651327b579cb64a454ab0f4c170b45b15f049a221",
	"IS_A_NUMBER":                            "1234",
	"NOT_A_NUMBER":                           "not_a_number",
}

func TestMain(m *testing.M) {
	for k, v := range testEnvVars {
		if err := os.Setenv(k, v); err != nil {
			fmt.Fprintf(os.Stderr, "setenv %s: %v\n", k, err)
			os.Exit(1)
		}
	}
	if err := os.Unsetenv("MISSING_ENV_VAR"); err != nil {
		fmt.Fprintf(os.Stderr, "unsetenv MISSING_ENV_VAR: %v\n", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// baseClientOptions are the options every datadir-backed integration client
// uses: the shared integration workspace, the Production environment, and
// no ~/.quonfig developer context (keeps the suite hermetic).
func baseClientOptions() []quonfig.Option {
	return []quonfig.Option{
		quonfig.WithDataDir(dataDir),
		quonfig.WithEnvironment("Production"),
		quonfig.WithQuonfigUserContext(false),
	}
}

// publicClient is one datadir-mode quonfig.Client shared by every case that
// needs no client-level option (no global context, no telemetry, no init
// overrides).
var (
	publicClientOnce sync.Once
	publicClient     *quonfig.Client
	publicClientErr  error
)

func mustPublicClient(t *testing.T) *quonfig.Client {
	t.Helper()
	publicClientOnce.Do(func() {
		publicClient, publicClientErr = quonfig.NewClient(baseClientOptions()...)
	})
	if publicClientErr != nil {
		t.Fatalf("quonfig.NewClient(WithDataDir) error: %v", publicClientErr)
	}
	return publicClient
}

// newPublicClient builds a fresh datadir-mode client with extra options
// (e.g. quonfig.WithGlobalContext) and closes it when the test ends.
func newPublicClient(t *testing.T, opts ...quonfig.Option) *quonfig.Client {
	t.Helper()
	c, err := quonfig.NewClient(append(baseClientOptions(), opts...)...)
	if err != nil {
		t.Fatalf("quonfig.NewClient error: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// contextSet builds a *quonfig.ContextSet with the public builder, one named
// context per entry. nil in, nil out.
func contextSet(contexts map[string]map[string]interface{}) *quonfig.ContextSet {
	if contexts == nil {
		return nil
	}
	cs := quonfig.NewContextSet()
	for name, values := range contexts {
		cs.WithNamedContextValues(name, values)
	}
	return cs
}

// newInitFailureClient is the hand-written half of the generated init-failure
// cases (contract: integration-test-data generators/src/targets/go.ts,
// initFailureClientCall). It builds the client with quonfig.NewClient and
// returns NewClient's (client, err) unchanged. onInitFailure "" leaves the
// option unset (the SDK default); any other spelling maps verbatim to the
// option of that name, deprecated aliases included. apiStatus != 0 stands up
// two local servers answering every request with that status and uses them as
// the legs; otherwise apiURLs are the legs. SSE, the fallback poller and
// telemetry are off.
func newInitFailureClient(t *testing.T, onInitFailure string, apiURLs []string, apiStatus int, initTimeoutMs int) (*quonfig.Client, error) {
	t.Helper()
	opts := []quonfig.Option{
		quonfig.WithSdkKey("itd-init-failure"),
		quonfig.WithInitTimeout(time.Duration(initTimeoutMs) * time.Millisecond),
		quonfig.WithSSE(false),
		quonfig.WithFallbackPoll(false, 0),
		quonfig.WithAllTelemetryDisabled(),
		quonfig.WithQuonfigUserContext(false),
	}
	switch onInitFailure {
	case "":
	case "fail":
		opts = append(opts, quonfig.WithOnInitFailure(quonfig.InitFailureFail))
	case "wait":
		opts = append(opts, quonfig.WithOnInitFailure(quonfig.InitFailureWait))
	case "continue":
		opts = append(opts, quonfig.WithOnInitFailure(quonfig.InitFailureContinue))
	case "raise":
		//lint:ignore SA1019 the deprecated alias is what the case exercises
		opts = append(opts, quonfig.WithOnInitFailure(quonfig.ReturnError)) //nolint:staticcheck // the alias is what the case exercises
	case "return":
		//lint:ignore SA1019 the deprecated alias is what the case exercises
		opts = append(opts, quonfig.WithOnInitFailure(quonfig.ReturnZeroValue)) //nolint:staticcheck // the alias is what the case exercises
	default:
		t.Fatalf("unknown on_init_failure %q", onInitFailure)
	}
	if apiStatus != 0 {
		apiURLs = nil
		for i := 0; i < 2; i++ {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(apiStatus)
			}))
			t.Cleanup(srv.Close)
			apiURLs = append(apiURLs, srv.URL)
		}
	}
	opts = append(opts, quonfig.WithAPIURLs(apiURLs))
	c, err := quonfig.NewClient(opts...)
	if c != nil {
		t.Cleanup(c.Close)
	}
	return c, err
}

// assertJSONValue compares a GetJSONValue result with the YAML's expected
// value by their JSON encodings, so int-vs-float64 decoding differences do
// not matter but a stringified payload (a JSON string instead of a native
// object) does.
func assertJSONValue(t *testing.T, want, got interface{}) {
	t.Helper()
	if s, isString := got.(string); isString {
		t.Fatalf("expected a native JSON value, got stringified payload %q", s)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal expected: %v", err)
	}
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal actual: %v", err)
	}
	var w, g interface{}
	_ = json.Unmarshal(wantJSON, &w)
	_ = json.Unmarshal(gotJSON, &g)
	if string(mustCanonicalJSON(w)) != string(mustCanonicalJSON(g)) {
		t.Errorf("JSON value mismatch:\n  got:  %s\n  want: %s", gotJSON, wantJSON)
	}
}

// mustCanonicalJSON re-encodes a decoded JSON value; encoding/json sorts map
// keys, so equal values encode identically.
func mustCanonicalJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

func mustJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
