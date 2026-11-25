package quonfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// SDK-level qf1 secret behaviour (qfg-8seuf.9, spec
// project/plans/secrets-qf1-spec.md 9.3, 13.3 R1, 13.5, 13.7). The crypto
// itself is pinned against every vector in internal/qf1; these tests drive the
// public Client. Test keys come from the shared vectors file in the sibling
// integration-test-data checkout (derived from public labels); none is
// committed here.

type sdkQf1Positive struct {
	Name       string `json:"name"`
	Keyset     string `json:"keyset"`
	Plaintext  string `json:"plaintext"`
	Ciphertext string `json:"ciphertext"`
}

type sdkQf1Vectors struct {
	PositiveEnv map[string]string `json:"positiveEnv"`
	Positive    []sdkQf1Positive  `json:"positive"`
}

func loadSDKQf1Vectors(t *testing.T) *sdkQf1Vectors {
	t.Helper()
	path := filepath.Join("..", "integration-test-data", "tests", "secrets", "qf1-vectors.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading qf1 vectors %s: %v", path, err)
	}
	var v sdkQf1Vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Positive) == 0 {
		t.Fatal("no positive vectors")
	}
	return &v
}

func positiveByName(t *testing.T, v *sdkQf1Vectors, name string) sdkQf1Positive {
	t.Helper()
	for _, p := range v.Positive {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no positive vector %q", name)
	return sdkQf1Positive{}
}

func mapEnvLookup(env map[string]string) EnvLookupFunc {
	return func(k string) (string, bool) {
		val, ok := env[k]
		return val, ok
	}
}

func stringDoc(key, typ, value string, confidential bool, decryptWith string) string {
	v := map[string]any{"type": "string", "value": value}
	if confidential {
		v["confidential"] = true
	}
	if decryptWith != "" {
		v["decryptWith"] = decryptWith
	}
	doc := map[string]any{
		"id": "id-" + key, "key": key, "type": typ, "valueType": "string",
		"default": map[string]any{"rules": []any{map[string]any{
			"criteria": []any{map[string]any{"operator": "ALWAYS_TRUE"}},
			"value":    v,
		}}},
	}
	b, _ := json.Marshal(doc)
	return string(b)
}

func writeQf1Workspace(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "quonfig.json"), []byte(`{"environments":["Production"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newDatadirClient(t *testing.T, dir string, env map[string]string) *Client {
	t.Helper()
	c, err := NewClient(
		WithDataDir(dir),
		WithEnvironment("Production"),
		WithQuonfigUserContext(false),
		WithAllTelemetryDisabled(),
		WithEnvLookup(mapEnvLookup(env)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// A secrets/ document shadows a same-key configs/ document (secrets/ loads
// last, the store is last-write-wins), and the directory, not the file's
// "type", makes it a secret.
func TestDatadirSecretsLoadLastAndDirectoryDecidesType(t *testing.T) {
	v := loadSDKQf1Vectors(t)
	short := positiveByName(t, v, "short-ascii")
	dir := writeQf1Workspace(t, map[string]string{
		"configs/shadowed.json": stringDoc("shadowed", "config", "deadbeef--00112233445566778899aabb--00112233445566778899aabbccddeeff", true, "some.key"),
		"secrets/shadowed.json": stringDoc("shadowed", "secret", short.Ciphertext, true, ""),
		// A secrets/ doc whose file claims "config" is still a secret.
		"secrets/mislabeled.json": stringDoc("mislabeled", "config", short.Ciphertext, true, ""),
		"configs/plain.json":      stringDoc("plain", "config", "plain-value", false, ""),
	})
	c := newDatadirClient(t, dir, v.PositiveEnv)

	for _, key := range []string{"shadowed", "mislabeled"} {
		got, ok, err := c.GetStringValue(key, nil)
		if err != nil || !ok || got != short.Plaintext {
			t.Fatalf("%s = %q ok=%v err=%v; want %q", key, got, ok, err, short.Plaintext)
		}
		details := c.EvaluateDetails(key, nil)
		if details.FlagMetadata["configType"] != "SECRET" {
			t.Fatalf("%s configType %v, want SECRET", key, details.FlagMetadata["configType"])
		}
	}
	if got, _, _ := c.GetStringValue("plain", nil); got != "plain-value" {
		t.Fatalf("plain = %q", got)
	}
}

// A process with no keyset env var at all boots, serves every non-secret key,
// and fails only the secret get (spec 9.3).
func TestNoKeysetEnvBootsAndFailsOnlyTheSecret(t *testing.T) {
	v := loadSDKQf1Vectors(t)
	short := positiveByName(t, v, "short-ascii")
	dir := writeQf1Workspace(t, map[string]string{
		"secrets/a.secret.json": stringDoc("a.secret", "secret", short.Ciphertext, true, ""),
		"configs/plain.json":    stringDoc("plain", "config", "plain-value", false, ""),
	})
	c := newDatadirClient(t, dir, map[string]string{})

	if got, ok, err := c.GetStringValue("plain", nil); err != nil || !ok || got != "plain-value" {
		t.Fatalf("plain = %q ok=%v err=%v", got, ok, err)
	}
	got, ok, err := c.GetStringValue("a.secret", nil)
	if !errors.Is(err, ErrUnableToDecrypt) || ok || got != "" {
		t.Fatalf("a.secret = %q ok=%v err=%v; want ErrUnableToDecrypt", got, ok, err)
	}
	if !strings.Contains(err.Error(), "qf1 keyset_missing") || !strings.Contains(err.Error(), short.Keyset) {
		t.Fatalf("error %q does not name class and keyset", err)
	}
	if strings.HasPrefix(got, "qf1:") {
		t.Fatal("returned the raw qf1 string")
	}
	// The failure is per get: evaluation details carry the error and the
	// client keeps serving.
	if d := c.EvaluateDetails("a.secret", nil); d.Reason != ReasonError || d.Value != nil {
		t.Fatalf("details = %+v, want an error with no value", d)
	}
	if got, _, _ := c.GetStringValue("plain", nil); got != "plain-value" {
		t.Fatalf("plain after failed secret = %q", got)
	}
}

// Rule R1: a value that still carries decryptWith fails with
// ErrUnableToDecrypt naming decryptWith, even when the key config it names
// exists and holds a valid old-format key, and is never returned as a string.
func TestDecryptWithValueFails(t *testing.T) {
	dir := writeQf1Workspace(t, map[string]string{
		"configs/old.secret.json": stringDoc("old.secret", "config", "deadbeef--00112233445566778899aabb--00112233445566778899aabbccddeeff", true, "old.key"),
		"configs/old.key.json":    stringDoc("old.key", "config", "c87ba22d8662282abe8a0e4651327b579cb64a454ab0f4c170b45b15f049a221", false, ""),
		// decryptWith without confidential is still the old format.
		"configs/old.nonconf.json": stringDoc("old.nonconf", "config", "deadbeef--00112233445566778899aabb--00112233445566778899aabbccddeeff", false, "old.key"),
	})
	c := newDatadirClient(t, dir, map[string]string{})
	for _, key := range []string{"old.secret", "old.nonconf"} {
		got, ok, err := c.GetStringValue(key, nil)
		if !errors.Is(err, ErrUnableToDecrypt) || ok || got != "" {
			t.Fatalf("%s = %q ok=%v err=%v; want ErrUnableToDecrypt", key, got, ok, err)
		}
		if !strings.Contains(err.Error(), "decryptWith") {
			t.Fatalf("%s error %q does not mention decryptWith", key, err)
		}
	}
}

// Wire mode: a type "secret" entry in the delivered envelope behaves like a
// config for getters and decrypts lazily on get.
func TestWireSecretEntry(t *testing.T) {
	v := loadSDKQf1Vectors(t)
	short := positiveByName(t, v, "short-ascii")
	body := envelopeJSON(stringDoc("wire.secret", "secret", short.Ciphertext, true, ""), stringDoc("wire.plain", "config", "p", false, ""))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(false),
		WithFallbackPoll(false, 0),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
		WithEnvLookup(mapEnvLookup(v.PositiveEnv)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Close)
	got, ok, err := c.GetStringValue("wire.secret", nil)
	if err != nil || !ok || got != short.Plaintext {
		t.Fatalf("wire.secret = %q ok=%v err=%v", got, ok, err)
	}
	if got, _, _ := c.GetStringValue("wire.plain", nil); got != "p" {
		t.Fatalf("wire.plain = %q", got)
	}
}

// Concurrent gets of every positive secret on a cold keyset cache, plus a
// missing-keyset secret beside them (run under -race).
func TestConcurrentSecretGetsColdCache(t *testing.T) {
	v := loadSDKQf1Vectors(t)
	files := map[string]string{}
	for _, p := range v.Positive {
		files["secrets/"+p.Name+".json"] = stringDoc(p.Name, "secret", p.Ciphertext, true, "")
	}
	files["secrets/missing.json"] = stringDoc("missing", "secret", strings.Replace(v.Positive[0].Ciphertext, v.Positive[0].Keyset+":", "QUONFIG_KEYSET_NOT_SET:", 1), true, "")
	c := newDatadirClient(t, writeQf1Workspace(t, files), v.PositiveEnv)

	var wg sync.WaitGroup
	errs := make(chan error, 32*(len(v.Positive)+1))
	for i := 0; i < 32; i++ {
		for _, p := range v.Positive {
			wg.Add(1)
			go func(p sdkQf1Positive) {
				defer wg.Done()
				got, _, err := c.GetStringValue(p.Name, nil)
				if err != nil || got != p.Plaintext {
					errs <- fmt.Errorf("%s: %q %v", p.Name, got, err)
				}
			}(p)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := c.GetStringValue("missing", nil); !errors.Is(err, ErrUnableToDecrypt) {
				errs <- fmt.Errorf("missing: %v", err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// Errors (message, %+v, %#v, every error in the chain, and the evaluation
// details) never contain key bytes, the keyset env value, the payload or the
// plaintext.
func TestSecretErrorsDoNotLeak(t *testing.T) {
	v := loadSDKQf1Vectors(t)
	short := positiveByName(t, v, "short-ascii")
	payload := short.Ciphertext[strings.LastIndex(short.Ciphertext, ":")+1:]
	// Tamper the last payload character so authentication fails (decrypt_failed).
	last := payload[len(payload)-1]
	swap := byte('A')
	if last == 'A' {
		swap = 'B'
	}
	tampered := short.Ciphertext[:len(short.Ciphertext)-1] + string(swap)

	// A keyset whose second entry is bad, next to a good one (keyset_invalid).
	badEnv := map[string]string{}
	for k, val := range v.PositiveEnv {
		badEnv[k] = val
	}
	badEnv[short.Keyset] = v.PositiveEnv[short.Keyset] + ",qf1-priv-tooshort"

	cases := map[string]struct {
		value string
		env   map[string]string
		class string
	}{
		"decrypt_failed": {tampered, v.PositiveEnv, "decrypt_failed"},
		"keyset_invalid": {short.Ciphertext, badEnv, "keyset_invalid"},
		"key_not_held":   {strings.Replace(short.Ciphertext, ":"+strings.Split(short.Ciphertext, ":")[2]+":", ":00000000:", 1), v.PositiveEnv, "key_not_held"},
		"keyset_missing": {short.Ciphertext, map[string]string{}, "keyset_missing"},
	}
	var forbidden []string
	for _, val := range v.PositiveEnv {
		for _, piece := range strings.Split(val, ",") {
			piece = strings.Trim(piece, " \t\r\n")
			if len(piece) > 25 {
				forbidden = append(forbidden, piece[9:25]) // inside the base64url body
			}
		}
	}
	forbidden = append(forbidden, payload[:20], payload[len(payload)-20:len(payload)-1], short.Plaintext)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := writeQf1Workspace(t, map[string]string{"secrets/s.json": stringDoc("s", "secret", tc.value, true, "")})
			c := newDatadirClient(t, dir, tc.env)
			details := c.EvaluateDetails("s", nil)
			_, _, err := c.GetStringValue("s", nil)
			if !errors.Is(err, ErrUnableToDecrypt) {
				t.Fatalf("err %v, want ErrUnableToDecrypt", err)
			}
			if !strings.Contains(err.Error(), "qf1 "+tc.class+":") {
				t.Fatalf("err %q does not carry class %s", err, tc.class)
			}
			texts := []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err), details.ErrorMessage, fmt.Sprintf("%+v", details)}
			var walk func(error)
			walk = func(e error) {
				if e == nil {
					return
				}
				texts = append(texts, e.Error(), fmt.Sprintf("%#v", e))
				switch u := e.(type) {
				case interface{ Unwrap() []error }:
					for _, inner := range u.Unwrap() {
						walk(inner)
					}
				case interface{ Unwrap() error }:
					walk(u.Unwrap())
				}
			}
			walk(err)
			for _, text := range texts {
				for _, f := range forbidden {
					if strings.Contains(text, f) {
						t.Fatalf("error text leaks %q: %s", f, text)
					}
				}
			}
		})
	}
}

// Telemetry redaction (spec 13.5): a secret's reportable value is the hash
// over the STORED string, also when the stored value lacks confidential.
func TestReportableValueForSecret(t *testing.T) {
	stored := &Value{Type: ValueTypeString, Value: "qf1:QUONFIG_KEYSET_TEST:b3f48931:AAAA"}
	got := reportableValueFor(stored, ConfigTypeSecret)
	if got == nil || !strings.HasPrefix(*got, reportableValuePrefix) {
		t.Fatalf("secret without confidential not redacted: %v", got)
	}
	want := reportableValueFor(&Value{Type: ValueTypeString, Value: stored.Value, Confidential: true}, ConfigTypeConfig)
	if want == nil || *got != *want {
		t.Fatalf("secret redaction %v differs from confidential redaction %v", got, want)
	}
	if reportableValueFor(&Value{Type: ValueTypeString, Value: "x"}, ConfigTypeConfig) != nil {
		t.Fatal("plain config value redacted")
	}
}

func newWireClient(t *testing.T, body string, env map[string]string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(
		WithSdkKey("test-key"),
		WithAPIURLs([]string{srv.URL}),
		WithSSE(false),
		WithFallbackPoll(false, 0),
		WithAllTelemetryDisabled(),
		WithInitTimeout(5*time.Second),
		WithEnvLookup(mapEnvLookup(env)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// Rule R1 wins over the secret path: a secret value that ALSO carries
// decryptWith is refused, never decrypted, in datadir and wire mode, even
// though its qf1 ciphertext would decrypt with the env held.
func TestDecryptWithOnSecretValueIsRefused(t *testing.T) {
	v := loadSDKQf1Vectors(t)
	short := positiveByName(t, v, "short-ascii")
	doc := stringDoc("both", "secret", short.Ciphertext, true, "some.key")
	clients := map[string]*Client{
		"datadir": newDatadirClient(t, writeQf1Workspace(t, map[string]string{"secrets/both.json": doc}), v.PositiveEnv),
		"wire":    newWireClient(t, envelopeJSON(doc), v.PositiveEnv),
	}
	for mode, c := range clients {
		got, ok, err := c.GetStringValue("both", nil)
		if !errors.Is(err, ErrUnableToDecrypt) || ok || got != "" {
			t.Fatalf("%s: both = %q ok=%v err=%v; want ErrUnableToDecrypt", mode, got, ok, err)
		}
		if !strings.Contains(err.Error(), "decryptWith") {
			t.Fatalf("%s: error %q does not name decryptWith", mode, err)
		}
	}
}

// The decrypted value keeps Confidential: true (so every redaction path that
// keys on it still applies), also when the stored secret lacks the flag.
func TestDecryptedSecretStaysConfidential(t *testing.T) {
	v := loadSDKQf1Vectors(t)
	short := positiveByName(t, v, "short-ascii")
	dir := writeQf1Workspace(t, map[string]string{
		"secrets/flagged.json":   stringDoc("flagged", "secret", short.Ciphertext, true, ""),
		"secrets/unflagged.json": stringDoc("unflagged", "secret", short.Ciphertext, false, ""),
	})
	c := newDatadirClient(t, dir, v.PositiveEnv)
	for _, key := range []string{"flagged", "unflagged"} {
		val, _, ok, err := c.EvaluateKey(key, nil)
		if err != nil || !ok || val == nil {
			t.Fatalf("%s: %v ok=%v err=%v", key, val, ok, err)
		}
		if !val.Confidential || val.Type != ValueTypeString || val.StringValue() != short.Plaintext {
			t.Fatalf("%s: got %+v, want a confidential string %q", key, *val, short.Plaintext)
		}
	}
}
