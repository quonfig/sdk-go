package qf1

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// The known-answer vectors are the shared integration-test-data copy of the
// spec's vectors.json (TEST KEYS ONLY, every key derived from a public label).
// They are read from the sibling checkout CI already uses for the generated
// fixtures; nothing key-shaped is committed to this repo.
var vectorsPath = filepath.Join("..", "..", "..", "integration-test-data", "tests", "secrets", "qf1-vectors.json")

type vectorKey struct {
	Label          string `json:"label"`
	PrivateKeyHex  string `json:"privateKeyHex"`
	PrivateKey     string `json:"privateKeyEntry"`
	PublicKeyHex   string `json:"publicKeyHex"`
	PublicKeyEntry string `json:"publicKeyEntry"`
	Fingerprint    string `json:"fingerprint"`
}

type vectorKeyset struct {
	Name       string      `json:"name"`
	PublicKeys []string    `json:"publicKeys"`
	EnvValue   string      `json:"envValue"`
	Keys       []vectorKey `json:"keys"`
}

type positiveCase struct {
	Name                   string `json:"name"`
	Keyset                 string `json:"keyset"`
	Fingerprint            string `json:"fingerprint"`
	RecipientPublicKeyHex  string `json:"recipientPublicKeyHex"`
	Plaintext              string `json:"plaintext"`
	PlaintextHex           string `json:"plaintextHex"`
	EphemeralPrivateKeyHex string `json:"ephemeralPrivateKeyHex"`
	EphemeralPublicKeyHex  string `json:"ephemeralPublicKeyHex"`
	SharedSecretHex        string `json:"sharedSecretHex"`
	HkdfSaltHex            string `json:"hkdfSaltHex"`
	HkdfInfoHex            string `json:"hkdfInfoHex"`
	PrkHex                 string `json:"prkHex"`
	OkmHex                 string `json:"okmHex"`
	AesKeyHex              string `json:"aesKeyHex"`
	NonceHex               string `json:"nonceHex"`
	AadASCII               string `json:"aadAscii"`
	AadHex                 string `json:"aadHex"`
	CiphertextAndTagHex    string `json:"ciphertextAndTagHex"`
	BlobHex                string `json:"blobHex"`
	Ciphertext             string `json:"ciphertext"`
}

type decryptOnlyCase struct {
	Name         string            `json:"name"`
	Ciphertext   string            `json:"ciphertext"`
	Env          map[string]string `json:"env"`
	Plaintext    string            `json:"plaintext"`
	PlaintextHex string            `json:"plaintextHex"`
}

type negativeCase struct {
	Name          string            `json:"name"`
	Ciphertext    string            `json:"ciphertext"`
	Env           map[string]string `json:"env"`
	ExpectedError string            `json:"expectedError"`
	MustReject    bool              `json:"mustReject"`
}

type keyEntryCase struct {
	Name                 string   `json:"name"`
	Kind                 string   `json:"kind"`
	Input                string   `json:"input"`
	Keyset               string   `json:"keyset"`
	ExpectedHex          *string  `json:"expectedHex"`
	ExpectedFingerprints []string `json:"expectedFingerprints"`
	ExpectedError        string   `json:"expectedError"`
}

type vectors struct {
	Constants struct {
		HkdfInfoASCII string `json:"hkdfInfoAscii"`
		HkdfInfoHex   string `json:"hkdfInfoHex"`
		OkmLength     int    `json:"okmLength"`
	} `json:"constants"`
	RFCChecks struct {
		RFC7748 struct {
			AlicePrivateHex string `json:"alicePrivateHex"`
			AlicePublicHex  string `json:"alicePublicHex"`
			BobPrivateHex   string `json:"bobPrivateHex"`
			BobPublicHex    string `json:"bobPublicHex"`
			SharedSecretHex string `json:"sharedSecretHex"`
		} `json:"rfc7748_6_1"`
		RFC5869 struct {
			IkmHex  string `json:"ikmHex"`
			SaltHex string `json:"saltHex"`
			InfoHex string `json:"infoHex"`
			Length  int    `json:"length"`
			PrkHex  string `json:"prkHex"`
			OkmHex  string `json:"okmHex"`
		} `json:"rfc5869_A_1"`
	} `json:"rfcChecks"`
	Keysets       []vectorKeyset    `json:"keysets"`
	PositiveEnv   map[string]string `json:"positiveEnv"`
	Positive      []positiveCase    `json:"positive"`
	DecryptOnly   []decryptOnlyCase `json:"decryptOnly"`
	Negative      []negativeCase    `json:"negative"`
	KeyEntryCases []keyEntryCase    `json:"keyEntryCases"`
}

var (
	loadOnce   sync.Once
	loadedVecs *vectors
	loadErr    error
)

func loadVectors(t *testing.T) *vectors {
	t.Helper()
	loadOnce.Do(func() {
		raw, err := os.ReadFile(vectorsPath)
		if err != nil {
			loadErr = err
			return
		}
		var v vectors
		if err := json.Unmarshal(raw, &v); err != nil {
			loadErr = err
			return
		}
		loadedVecs = &v
	})
	if loadErr != nil {
		t.Fatalf("reading qf1 vectors %s (the sibling integration-test-data checkout CI uses): %v", vectorsPath, loadErr)
	}
	return loadedVecs
}

// mapLookup is an env lookup over exactly the given map; anything not in it is
// unset. Tests never read the machine's real QUONFIG_KEYSET* variables.
func mapLookup(env map[string]string) LookupFunc {
	return func(name string) (string, bool) {
		v, ok := env[name]
		return v, ok
	}
}

// recordingLookup records every env var name it is asked for.
type recordingLookup struct {
	mu    sync.Mutex
	env   map[string]string
	asked []string
}

func (r *recordingLookup) lookup(name string) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.asked = append(r.asked, name)
	v, ok := r.env[name]
	return v, ok
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}
	return b
}

func classOf(err error) Class {
	var qe *Error
	if errors.As(err, &qe) {
		return qe.Class
	}
	return ""
}

// secretFragments lists every string an error must never contain for this
// env: each private entry, its base64url body, and the hex of its bytes.
func secretFragments(t *testing.T, v *vectors, env map[string]string) []string {
	t.Helper()
	var out []string
	for _, val := range env {
		for _, piece := range strings.Split(val, ",") {
			piece = strings.Trim(piece, " \t\r\n")
			if body, ok := strings.CutPrefix(piece, "qf1-priv-"); ok && len(body) >= 16 {
				out = append(out, body[:16])
			}
		}
	}
	for _, ks := range v.Keysets {
		for _, k := range ks.Keys {
			out = append(out, strings.TrimPrefix(k.PrivateKey, "qf1-priv-")[:16], k.PrivateKeyHex[:16])
		}
	}
	return out
}

func assertNoLeak(t *testing.T, err error, forbidden ...string) {
	t.Helper()
	texts := []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)}
	for e := errors.Unwrap(err); e != nil; e = errors.Unwrap(e) {
		texts = append(texts, e.Error())
	}
	for _, text := range texts {
		for _, f := range forbidden {
			if f != "" && strings.Contains(text, f) {
				t.Errorf("error text leaks %q: %s", f, text)
			}
		}
	}
}

func TestRFC7748X25519(t *testing.T) {
	v := loadVectors(t)
	r := v.RFCChecks.RFC7748
	priv, err := ecdh.X25519().NewPrivateKey(mustHex(t, r.AlicePrivateHex))
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(priv.PublicKey().Bytes()); got != r.AlicePublicHex {
		t.Fatalf("alice public %s, want %s", got, r.AlicePublicHex)
	}
	shared, err := x25519(mustHex(t, r.AlicePrivateHex), mustHex(t, r.BobPublicHex))
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(shared); got != r.SharedSecretHex {
		t.Fatalf("shared %s, want %s", got, r.SharedSecretHex)
	}
}

func TestRFC5869HKDF(t *testing.T) {
	v := loadVectors(t)
	r := v.RFCChecks.RFC5869
	prk := hkdfExtract(mustHex(t, r.SaltHex), mustHex(t, r.IkmHex))
	if got := hex.EncodeToString(prk); got != r.PrkHex {
		t.Fatalf("prk %s, want %s", got, r.PrkHex)
	}
	okm := hkdfExpand(prk, mustHex(t, r.InfoHex), r.Length)
	if got := hex.EncodeToString(okm); got != r.OkmHex {
		t.Fatalf("okm %s, want %s", got, r.OkmHex)
	}
}

func TestConstants(t *testing.T) {
	v := loadVectors(t)
	if string(hkdfInfo) != v.Constants.HkdfInfoASCII || hex.EncodeToString(hkdfInfo) != v.Constants.HkdfInfoHex {
		t.Fatalf("hkdf info %q does not match vectors", hkdfInfo)
	}
	if okmLength != v.Constants.OkmLength {
		t.Fatalf("okm length %d, want %d", okmLength, v.Constants.OkmLength)
	}
}

// TestVectorKeysets parses every keyset's envValue and checks each derived
// public key and fingerprint against the vectors.
func TestVectorKeysets(t *testing.T) {
	v := loadVectors(t)
	if len(v.Keysets) == 0 {
		t.Fatal("no keysets in vectors")
	}
	for _, ks := range v.Keysets {
		t.Run(ks.Name, func(t *testing.T) {
			parsed, err := ParseKeyset(ks.Name, ks.EnvValue, true)
			if err != nil {
				t.Fatalf("ParseKeyset: %v", err)
			}
			var want []string
			for _, k := range ks.Keys {
				want = append(want, k.Fingerprint)
				held, ok := parsed.keys[k.Fingerprint]
				if !ok {
					t.Fatalf("fingerprint %s not derived", k.Fingerprint)
				}
				if got := hex.EncodeToString(held.public); got != k.PublicKeyHex {
					t.Errorf("%s public %s, want %s", k.Label, got, k.PublicKeyHex)
				}
				priv, err := ParsePrivateKeyEntry(k.PrivateKey)
				if err != nil {
					t.Fatalf("%s: %v", k.Label, err)
				}
				if got := hex.EncodeToString(priv); got != k.PrivateKeyHex {
					t.Errorf("%s private bytes mismatch", k.Label)
				}
			}
			sort.Strings(want)
			if got := parsed.Fingerprints(); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("fingerprints %v, want %v", got, want)
			}
		})
	}
}

// TestPositiveDerivation reproduces every intermediate of every positive case
// (shared secret, salt, PRK, OKM, key, nonce, AAD, blob layout).
func TestPositiveDerivation(t *testing.T) {
	v := loadVectors(t)
	for _, c := range v.Positive {
		t.Run(c.Name, func(t *testing.T) {
			eph := mustHex(t, c.EphemeralPublicKeyHex)
			pubR := mustHex(t, c.RecipientPublicKeyHex)
			shared, err := x25519(mustHex(t, c.EphemeralPrivateKeyHex), pubR)
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(shared) != c.SharedSecretHex {
				t.Fatalf("shared secret mismatch")
			}
			if got := hex.EncodeToString(hkdfSalt(eph, pubR)); got != c.HkdfSaltHex {
				t.Fatalf("salt %s, want %s", got, c.HkdfSaltHex)
			}
			if hex.EncodeToString(hkdfInfo) != c.HkdfInfoHex {
				t.Fatalf("info mismatch")
			}
			prk := hkdfExtract(hkdfSalt(eph, pubR), shared)
			if hex.EncodeToString(prk) != c.PrkHex {
				t.Fatalf("prk mismatch")
			}
			key, nonce := deriveKeyNonce(shared, eph, pubR)
			if hex.EncodeToString(append(append([]byte{}, key...), nonce...)) != c.OkmHex {
				t.Fatalf("okm mismatch")
			}
			if hex.EncodeToString(key) != c.AesKeyHex || hex.EncodeToString(nonce) != c.NonceHex {
				t.Fatalf("key/nonce mismatch")
			}
			ct, err := Parse(c.Ciphertext)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if ct.Keyset != c.Keyset || ct.Fingerprint != c.Fingerprint {
				t.Fatalf("parsed header %s/%s", ct.Keyset, ct.Fingerprint)
			}
			if string(ct.aad) != c.AadASCII || hex.EncodeToString(ct.aad) != c.AadHex {
				t.Fatalf("aad %q, want %q", ct.aad, c.AadASCII)
			}
			if !bytes.Equal(ct.ephemeral, eph) || hex.EncodeToString(ct.sealed) != c.CiphertextAndTagHex {
				t.Fatalf("payload split mismatch")
			}
			if hex.EncodeToString(append(append([]byte{}, ct.ephemeral...), ct.sealed...)) != c.BlobHex {
				t.Fatalf("blob mismatch")
			}
		})
	}
}

func TestPositiveDecrypt(t *testing.T) {
	v := loadVectors(t)
	if len(v.Positive) == 0 {
		t.Fatal("no positive vectors")
	}
	r := NewReader(mapLookup(v.PositiveEnv))
	for _, c := range v.Positive {
		t.Run(c.Name, func(t *testing.T) {
			got, err := r.Decrypt(c.Ciphertext)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if got != c.Plaintext || hex.EncodeToString([]byte(got)) != c.PlaintextHex {
				t.Fatalf("plaintext %q, want %q", got, c.Plaintext)
			}
		})
	}
}

func TestDecryptOnly(t *testing.T) {
	v := loadVectors(t)
	if len(v.DecryptOnly) == 0 {
		t.Fatal("no decryptOnly vectors")
	}
	for _, c := range v.DecryptOnly {
		t.Run(c.Name, func(t *testing.T) {
			got, err := NewReader(mapLookup(c.Env)).Decrypt(c.Ciphertext)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if got != c.Plaintext || hex.EncodeToString([]byte(got)) != c.PlaintextHex {
				t.Fatalf("plaintext %q, want %q", got, c.Plaintext)
			}
		})
	}
}

// TestNegative requires the exact error class for every negative vector under
// the spec's reader order (3.5, 6.2, 8). The mustReject ones are authentic
// except for the rule they test, so a reader missing that rule decrypts them.
func TestNegative(t *testing.T) {
	v := loadVectors(t)
	if len(v.Negative) == 0 {
		t.Fatal("no negative vectors")
	}
	for _, c := range v.Negative {
		t.Run(c.Name, func(t *testing.T) {
			rec := &recordingLookup{env: c.Env}
			got, err := NewReader(rec.lookup).Decrypt(c.Ciphertext)
			if err == nil {
				t.Fatalf("decrypted to %q, want %s (mustReject=%v)", got, c.ExpectedError, c.MustReject)
			}
			if got != "" {
				t.Fatalf("returned %q alongside an error", got)
			}
			if cls := classOf(err); string(cls) != c.ExpectedError {
				t.Fatalf("class %q, want %q (%v)", cls, c.ExpectedError, err)
			}
			if !strings.HasPrefix(err.Error(), "qf1 "+c.ExpectedError+": ") {
				t.Fatalf("message %q does not start with qf1 %s:", err.Error(), c.ExpectedError)
			}
			// Parse failures read no env var at all; every other class reads
			// exactly the one named by the header.
			switch Class(c.ExpectedError) {
			case Malformed, UnsupportedVersion, InvalidKeysetName:
				if len(rec.asked) != 0 {
					t.Fatalf("read env %q for a %s value", rec.asked, c.ExpectedError)
				}
			default:
				ct, perr := Parse(c.Ciphertext)
				if perr != nil {
					t.Fatalf("Parse: %v", perr)
				}
				for _, name := range rec.asked {
					if name != ct.Keyset {
						t.Fatalf("read env %q, only %q allowed", name, ct.Keyset)
					}
				}
			}
			forbidden := secretFragments(t, v, c.Env)
			if idx := strings.LastIndex(c.Ciphertext, ":"); idx >= 0 && len(c.Ciphertext)-idx > 20 {
				forbidden = append(forbidden, c.Ciphertext[idx+1:idx+21])
			}
			assertNoLeak(t, err, forbidden...)
		})
	}
}

// TestMustRejectCount guards against a vectors file (or loader) that silently
// drops the accepting negative cases.
func TestMustRejectCount(t *testing.T) {
	v := loadVectors(t)
	n := 0
	for _, c := range v.Negative {
		if c.MustReject {
			n++
		}
	}
	if n < 30 {
		t.Fatalf("only %d mustReject vectors", n)
	}
}

func TestKeyEntryCases(t *testing.T) {
	v := loadVectors(t)
	ran := 0
	for _, c := range v.KeyEntryCases {
		c := c
		switch c.Kind {
		case "privateKeyEntry":
			ran++
			t.Run(c.Name, func(t *testing.T) {
				got, err := ParsePrivateKeyEntry(c.Input)
				if c.ExpectedError != "" {
					if err == nil {
						t.Fatalf("accepted, want %s", c.ExpectedError)
					}
					if string(classOf(err)) != c.ExpectedError {
						t.Fatalf("class %q, want %q", classOf(err), c.ExpectedError)
					}
					assertNoLeak(t, err, strings.TrimPrefix(c.Input, "qf1-priv-"))
					return
				}
				if err != nil {
					t.Fatalf("ParsePrivateKeyEntry: %v", err)
				}
				if c.ExpectedHex == nil || hex.EncodeToString(got) != *c.ExpectedHex {
					t.Fatalf("bytes mismatch")
				}
			})
		case "envValue":
			ran++
			t.Run(c.Name, func(t *testing.T) {
				ks, err := ParseKeyset(c.Keyset, c.Input, true)
				if c.ExpectedError != "" {
					if err == nil {
						t.Fatalf("accepted, want %s", c.ExpectedError)
					}
					if string(classOf(err)) != c.ExpectedError {
						t.Fatalf("class %q, want %q (%v)", classOf(err), c.ExpectedError, err)
					}
					if !strings.Contains(err.Error(), c.Keyset) {
						t.Fatalf("error %q does not name keyset %s", err, c.Keyset)
					}
					assertNoLeak(t, err, secretFragments(t, v, map[string]string{c.Keyset: c.Input})...)
					return
				}
				if err != nil {
					t.Fatalf("ParseKeyset: %v", err)
				}
				want := append([]string{}, c.ExpectedFingerprints...)
				sort.Strings(want)
				if got := ks.Fingerprints(); strings.Join(got, ",") != strings.Join(want, ",") {
					t.Fatalf("fingerprints %v, want %v", got, want)
				}
			})
		case "publicKeyEntry":
			// A reader never parses public key entries: it derives public keys
			// from the private entries it holds (spec 4, 6.2). Writers and
			// qfg-verify own these cases.
		default:
			t.Errorf("unknown keyEntryCases kind %q", c.Kind)
		}
	}
	if ran < 15 {
		t.Fatalf("only %d reader key-entry cases ran", ran)
	}
}

// TestUnsetKeysetIsMissing covers 6.2 step 1 (unset), which the envValue
// cases cannot express.
func TestUnsetKeysetIsMissing(t *testing.T) {
	_, err := ParseKeyset("QUONFIG_KEYSET_X", "", false)
	if classOf(err) != KeysetMissing {
		t.Fatalf("class %q, want keyset_missing", classOf(err))
	}
	if !strings.Contains(err.Error(), "QUONFIG_KEYSET_X") {
		t.Fatalf("error %q does not name the keyset", err)
	}
}

// TestKeyNotHeldNamesFingerprints pins the 9.2 text rule: the needed
// fingerprint and the fingerprints held are named.
func TestKeyNotHeldNamesFingerprints(t *testing.T) {
	v := loadVectors(t)
	for _, c := range v.Negative {
		if c.ExpectedError != string(KeyNotHeld) {
			continue
		}
		ct, err := Parse(c.Ciphertext)
		if err != nil {
			t.Fatal(err)
		}
		_, err = NewReader(mapLookup(c.Env)).Decrypt(c.Ciphertext)
		ks, kerr := ParseKeyset(ct.Keyset, c.Env[ct.Keyset], true)
		if kerr != nil {
			t.Fatal(kerr)
		}
		msg := err.Error()
		if !strings.Contains(msg, ct.Keyset) || !strings.Contains(msg, ct.Fingerprint) {
			t.Errorf("%s: %q does not name keyset and needed fingerprint", c.Name, msg)
		}
		for _, fp := range ks.Fingerprints() {
			if !strings.Contains(msg, fp) {
				t.Errorf("%s: %q does not name held fingerprint %s", c.Name, msg, fp)
			}
		}
	}
}

// TestReaderCacheFollowsEnvValue: a cached keyset is reused while the env
// value is unchanged, and re-parsed when it changes (or appears).
func TestReaderCacheFollowsEnvValue(t *testing.T) {
	v := loadVectors(t)
	c := v.Positive[0]
	env := map[string]string{}
	var mu sync.Mutex
	r := NewReader(func(name string) (string, bool) {
		mu.Lock()
		defer mu.Unlock()
		val, ok := env[name]
		return val, ok
	})
	if _, err := r.Decrypt(c.Ciphertext); classOf(err) != KeysetMissing {
		t.Fatalf("want keyset_missing first, got %v", err)
	}
	mu.Lock()
	env[c.Keyset] = "not-a-key"
	mu.Unlock()
	if _, err := r.Decrypt(c.Ciphertext); classOf(err) != KeysetInvalid {
		t.Fatalf("want keyset_invalid for a bad value, got %v", err)
	}
	mu.Lock()
	env[c.Keyset] = v.PositiveEnv[c.Keyset]
	mu.Unlock()
	got, err := r.Decrypt(c.Ciphertext)
	if err != nil || got != c.Plaintext {
		t.Fatalf("after setting env: %q %v", got, err)
	}
}

// TestReaderConcurrentColdCache decrypts every positive vector from many
// goroutines at once on a cold cache (run under -race).
func TestReaderConcurrentColdCache(t *testing.T) {
	v := loadVectors(t)
	r := NewReader(mapLookup(v.PositiveEnv))
	var wg sync.WaitGroup
	errs := make(chan error, 64*len(v.Positive))
	for i := 0; i < 64; i++ {
		for _, c := range v.Positive {
			wg.Add(1)
			go func(c positiveCase) {
				defer wg.Done()
				got, err := r.Decrypt(c.Ciphertext)
				if err != nil {
					errs <- err
					return
				}
				if got != c.Plaintext {
					errs <- fmt.Errorf("%s: got %q", c.Name, got)
				}
			}(c)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// encodings returns the forms in which bytes b could land in an error text:
// raw, lowercase and uppercase hex, standard and URL base64 (padded and not).
func encodings(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	return []string{
		string(b),
		hex.EncodeToString(b),
		strings.ToUpper(hex.EncodeToString(b)),
		base64.StdEncoding.EncodeToString(b),
		base64.RawStdEncoding.EncodeToString(b),
		base64.URLEncoding.EncodeToString(b),
		base64.RawURLEncoding.EncodeToString(b),
		fmt.Sprintf("%v", b),
		fmt.Sprintf("%q", b),
	}
}

// derivedSecrets recomputes, outside the reader, every secret intermediate a
// reader holds while it builds an authentication or UTF-8 error: the shared
// secret, PRK, OKM, AES key, nonce, and (when the tag verifies) the plaintext.
func derivedSecrets(t *testing.T, value string, env map[string]string) (secrets [][]byte, plaintext []byte) {
	t.Helper()
	ct, err := Parse(value)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	envVal, ok := env[ct.Keyset]
	ks, err := ParseKeyset(ct.Keyset, envVal, ok)
	if err != nil {
		t.Fatalf("ParseKeyset: %v", err)
	}
	held, ok := ks.keys[ct.Fingerprint]
	if !ok {
		t.Fatalf("fingerprint %s not held", ct.Fingerprint)
	}
	shared, err := x25519With(held.private, ct.ephemeral)
	if err != nil {
		t.Fatalf("x25519: %v", err)
	}
	prk := hkdfExtract(hkdfSalt(ct.ephemeral, held.public), shared)
	okm := hkdfExpand(prk, hkdfInfo, okmLength)
	key, nonce := okm[:32], okm[32:]
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	plain, _ := gcm.Open(nil, nonce, ct.sealed, ct.aad)
	return [][]byte{shared, prk, okm, key, nonce, held.private.Bytes()}, plain
}

func forbiddenFrom(secrets [][]byte, plaintext []byte) []string {
	var out []string
	for _, s := range secrets {
		out = append(out, encodings(s)...)
		// A 6-byte slice of any intermediate is a leak too: hex at every
		// offset, base64 at the 3-byte boundaries where its text aligns.
		for i := 0; i+6 <= len(s); i++ {
			out = append(out, hex.EncodeToString(s[i:i+6]))
			if i%3 == 0 {
				out = append(out, base64.RawStdEncoding.EncodeToString(s[i:i+6]), base64.RawURLEncoding.EncodeToString(s[i:i+6]))
			}
		}
	}
	out = append(out, encodings(plaintext)...)
	// Any 6-byte window of the plaintext, as text or hex, is a leak too.
	for i := 0; i+6 <= len(plaintext); i++ {
		w := plaintext[i : i+6]
		out = append(out, string(w), hex.EncodeToString(w))
	}
	return out
}

// TestDecryptFailedErrorsDoNotLeakDerivedSecrets: for every decrypt_failed
// vector (authentication failure or non-UTF-8 plaintext) the error contains
// none of the shared secret, PRK, OKM, AES key, nonce, private key or the
// plaintext bytes, in raw, hex or base64 form.
func TestDecryptFailedErrorsDoNotLeakDerivedSecrets(t *testing.T) {
	v := loadVectors(t)
	n := 0
	for _, c := range v.Negative {
		if c.ExpectedError != string(DecryptFailed) {
			continue
		}
		n++
		t.Run(c.Name, func(t *testing.T) {
			secrets, plain := derivedSecrets(t, c.Ciphertext, c.Env)
			_, err := NewReader(mapLookup(c.Env)).Decrypt(c.Ciphertext)
			if classOf(err) != DecryptFailed {
				t.Fatalf("class %q, want decrypt_failed", classOf(err))
			}
			assertNoLeak(t, err, forbiddenFrom(secrets, plain)...)
		})
	}
	if n < 8 {
		t.Fatalf("only %d decrypt_failed vectors", n)
	}
}

// sealForTest encrypts plaintext bytes to the first key of a vector keyset,
// exactly as spec 7 does but with no writer checks (test-only; the SDK ships
// no writer). The ephemeral key is fixed so the test is deterministic.
func sealForTest(t *testing.T, ks vectorKeyset, plaintext []byte) string {
	t.Helper()
	ephPriv, err := ecdh.X25519().NewPrivateKey(bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}
	pubR := mustHex(t, ks.Keys[0].PublicKeyHex)
	shared, err := x25519(ephPriv.Bytes(), pubR)
	if err != nil {
		t.Fatal(err)
	}
	eph := ephPriv.PublicKey().Bytes()
	key, nonce := deriveKeyNonce(shared, eph, pubR)
	header := "qf1:" + ks.Name + ":" + ks.Keys[0].Fingerprint + ":"
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	sealed := gcm.Seal(nil, nonce, plaintext, []byte(header))
	return header + base64.RawURLEncoding.EncodeToString(append(append([]byte{}, eph...), sealed...))
}

// TestNonUTF8PlaintextErrorDoesNotLeakPlaintext seals a known sentinel
// followed by invalid UTF-8 and requires the decrypt_failed error to carry
// no part of it.
func TestNonUTF8PlaintextErrorDoesNotLeakPlaintext(t *testing.T) {
	v := loadVectors(t)
	ks := v.Keysets[0]
	env := map[string]string{ks.Name: ks.EnvValue}
	for name, plain := range map[string][]byte{
		"trailing-ff":       append([]byte("SENTINEL-PLAINTEXT-7c1d"), 0xff),
		"encoded-surrogate": append([]byte("SENTINEL-PLAINTEXT-7c1d"), 0xed, 0xa0, 0x80),
		"overlong-nul":      append([]byte("SENTINEL-PLAINTEXT-7c1d"), 0xc0, 0x80),
	} {
		t.Run(name, func(t *testing.T) {
			value := sealForTest(t, ks, plain)
			secrets, got := derivedSecrets(t, value, env)
			if !bytes.Equal(got, plain) {
				t.Fatalf("test sealer round trip failed")
			}
			_, err := NewReader(mapLookup(env)).Decrypt(value)
			if classOf(err) != DecryptFailed {
				t.Fatalf("class %q, want decrypt_failed (%v)", classOf(err), err)
			}
			assertNoLeak(t, err, append(forbiddenFrom(secrets, plain), "SENTINEL", "7c1d")...)
		})
	}
}

// TestInvalidKeysetNameIsNotQuoted: a keyset field that is not a valid name
// is never echoed (a pasted private key or plaintext would land in logs);
// the error says only that it is invalid and how long it is.
func TestInvalidKeysetNameIsNotQuoted(t *testing.T) {
	v := loadVectors(t)
	payload := v.Positive[0].Ciphertext[strings.LastIndex(v.Positive[0].Ciphertext, ":")+1:]
	for _, name := range []string{
		"SENTINELsecretpastedhere",
		"QUONFIG_KEYSET_SENTINEL\n",
		"HOME",
		strings.Repeat("S", 100),
	} {
		_, err := NewReader(mapLookup(nil)).Decrypt("qf1:" + name + ":b3f48931:" + payload)
		if classOf(err) != InvalidKeysetName {
			t.Fatalf("%q: class %q", name, classOf(err))
		}
		if !strings.Contains(err.Error(), fmt.Sprintf("%d characters", len(name))) {
			t.Fatalf("%q: error %q does not give the length", name, err)
		}
		assertNoLeak(t, err, "SENTINEL", "HOME", "SSSS", strings.TrimSpace(name))
	}
}

// TestValidKeysetNameTruncatedTo64: a valid keyset name longer than 64
// characters is echoed only as its first 64 characters.
func TestValidKeysetNameTruncatedTo64(t *testing.T) {
	v := loadVectors(t)
	c := v.Positive[0]
	long := "QUONFIG_KEYSET_" + strings.Repeat("A", 49) + "_TAILMARKER"
	if !ValidKeysetName(long) || len(long) <= 64 {
		t.Fatal("bad test name")
	}
	value := strings.Replace(c.Ciphertext, ":"+c.Keyset+":", ":"+long+":", 1)
	checks := map[string]map[string]string{
		"keyset_missing": {},
		"keyset_invalid": {long: "not-a-key"},
		"key_not_held":   {long: v.PositiveEnv["QUONFIG_KEYSET"]},
	}
	for want, env := range checks {
		_, err := NewReader(mapLookup(env)).Decrypt(value)
		if string(classOf(err)) != want {
			t.Fatalf("class %q, want %s (%v)", classOf(err), want, err)
		}
		if !strings.Contains(err.Error(), long[:64]) {
			t.Fatalf("%s: %q does not name the truncated keyset", want, err)
		}
		assertNoLeak(t, err, "TAILMARKER", long[:65])
	}
}

func TestDisplayKeysetName(t *testing.T) {
	if got := displayKeysetName("QUONFIG_KEYSET_PROD"); got != "QUONFIG_KEYSET_PROD" {
		t.Fatalf("got %q", got)
	}
	long := "QUONFIG_KEYSET_" + strings.Repeat("B", 80)
	if got := displayKeysetName(long); got != long[:64]+"..." {
		t.Fatalf("got %q", got)
	}
}

// TestReaderCacheDoesNotHoldRawEnvValue: the cache compares a digest of the
// env value, never a second copy of the private entries.
func TestReaderCacheDoesNotHoldRawEnvValue(t *testing.T) {
	v := loadVectors(t)
	c := v.Positive[0]
	r := NewReader(mapLookup(v.PositiveEnv))
	if _, err := r.Decrypt(c.Ciphertext); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, entry := range r.cache {
		dump := fmt.Sprintf("%#v", entry)
		for _, piece := range strings.Split(v.PositiveEnv[name], ",") {
			piece = strings.Trim(piece, " \t\r\n")
			if piece != "" && strings.Contains(dump, piece) {
				t.Fatalf("cache entry for %s holds the raw env entry", name)
			}
		}
	}
}

// TestFIPSOnlyRuntimeRefusal: under GODEBUG=fips140=only crypto/ecdh refuses
// X25519. That must fail closed with an error naming the runtime restriction,
// not blaming the key entry. GODEBUG fips140 is read at process start, so the
// check runs in a child test process.
func TestFIPSOnlyRuntimeRefusal(t *testing.T) {
	if os.Getenv("QF1_FIPS_CHILD") == "1" {
		v := loadVectors(t)
		c := v.Positive[0]
		if _, err := ecdh.X25519().NewPrivateKey(make([]byte, 32)); err == nil {
			t.Skip("this toolchain does not restrict X25519 under fips140=only")
		}
		got, err := NewReader(mapLookup(v.PositiveEnv)).Decrypt(c.Ciphertext)
		if err == nil || got != "" {
			t.Fatalf("decrypted %q under fips140=only", got)
		}
		msg := err.Error()
		if strings.Contains(msg, "is not a qf1 private key") || classOf(err) == KeysetInvalid {
			t.Fatalf("blames the key entry: %s", msg)
		}
		if !strings.Contains(msg, "X25519") || !strings.Contains(msg, "fips140") {
			t.Fatalf("does not name the runtime restriction: %s", msg)
		}
		fmt.Println("FIPS_CHILD_OK:", msg)
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestFIPSOnlyRuntimeRefusal$", "-test.v")
	cmd.Env = append(os.Environ(), "QF1_FIPS_CHILD=1", "GODEBUG=fips140=only")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child failed: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "--- SKIP") {
		t.Skipf("child skipped:\n%s", out)
	}
	if !strings.Contains(string(out), "FIPS_CHILD_OK:") {
		t.Fatalf("child did not run the check:\n%s", out)
	}
	t.Logf("%s", out)
}
