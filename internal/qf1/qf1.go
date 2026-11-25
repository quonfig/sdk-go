// Package qf1 is the read side of the qf1 secret format: it parses a stored
// "qf1:<KEYSET>:<FINGERPRINT>:<PAYLOAD>" value, finds the private key by
// (keyset name, fingerprint) in the env var named by the keyset, and decrypts
// it with X25519, HKDF-SHA256 and AES-256-GCM.
//
// Spec: project/plans/secrets-qf1-spec.md (sections 2 to 9). Section numbers
// in comments refer to it. There is no writer here: SDKs only read.
package qf1

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// Class is a normative reader error class (spec 9.1).
type Class string

// Reader error classes (spec 9.1).
const (
	Malformed           Class = "malformed"
	UnsupportedVersion  Class = "unsupported_version"
	InvalidKeysetName   Class = "invalid_keyset_name"
	KeysetMissing       Class = "keyset_missing"
	KeysetInvalid       Class = "keyset_invalid"
	KeyNotHeld          Class = "key_not_held"
	InvalidEphemeralKey Class = "invalid_ephemeral_key"
	DecryptFailed       Class = "decrypt_failed"
	InvalidKeyEntry     Class = "invalid_key_entry"
)

// Error is a qf1 failure. Its text is "qf1 <class>: <detail>" (spec 9.2) and
// never contains key material, any part of a keyset env value, the payload or
// the plaintext.
type Error struct {
	Class  Class
	detail string
}

func (e *Error) Error() string { return "qf1 " + string(e.Class) + ": " + e.detail }

func fail(class Class, format string, args ...any) *Error {
	return &Error{Class: class, detail: fmt.Sprintf(format, args...)}
}

// LookupFunc looks up an environment variable by name.
type LookupFunc func(name string) (string, bool)

const (
	version           = "qf1"
	privateKeyPrefix  = "qf1-priv-"
	keyLength         = 32
	tagLength         = 16
	minPayloadLength  = keyLength + tagLength
	okmLength         = 44
	keysetNamePattern = `^QUONFIG_KEYSET(_[A-Z0-9]+)*$`
	envTrimSet        = " \t\r\n" // spec 6.2: ONLY these, never a general trim
)

var (
	hkdfInfo = []byte("quonfig.com/qf1/X25519")

	// Full-string matches (spec 0): \A and \z, never line anchors.
	keysetNameRE  = regexp.MustCompile(`\AQUONFIG_KEYSET(?:_[A-Z0-9]+)*\z`)
	fingerprintRE = regexp.MustCompile(`\A[0-9a-f]{8}\z`)
	otherVersion  = regexp.MustCompile(`\Aqf[1-9][0-9]*:`)
	base64urlRE   = regexp.MustCompile(`\A[A-Za-z0-9_-]+\z`)
)

// ValidKeysetName reports whether name is a keyset name (spec 2).
func ValidKeysetName(name string) bool { return keysetNameRE.MatchString(name) }

// decodeBase64url is the strict canonical unpadded decoder of spec 3.4. The
// alphabet check comes first: Go's decoder silently skips CR and LF.
func decodeBase64url(s string) ([]byte, bool) {
	if s == "" || !base64urlRE.MatchString(s) || len(s)%4 == 1 {
		return nil, false
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, false
	}
	if base64.RawURLEncoding.EncodeToString(b) != s {
		return nil, false // non-zero unused trailing bits
	}
	return b, true
}

// Ciphertext is a parsed qf1 value (spec 3.5).
type Ciphertext struct {
	Keyset      string
	Fingerprint string
	aad         []byte // "qf1:<KEYSET>:<FINGERPRINT>:" (spec 5.4)
	ephemeral   []byte // E, 32 bytes as transmitted
	sealed      []byte // C || T
}

// quoteName renders a writer-controlled keyset name for an error: at most 64
// characters, control characters escaped (spec 9.2).
func quoteName(name string) string {
	if len(name) > 64 {
		name = name[:64]
	}
	return strconv.QuoteToASCII(name)
}

// Parse applies spec 3.5 in order; the first failing step decides the class.
// It reads no environment.
func Parse(v string) (*Ciphertext, error) {
	if !strings.HasPrefix(v, version+":") {
		if otherVersion.MatchString(v) {
			return nil, fail(UnsupportedVersion, "value uses an unsupported qf version; this SDK reads qf1 only")
		}
		return nil, fail(Malformed, "value is not a qf1 string")
	}
	fields := strings.Split(v, ":") // keeps empty fields, trailing ones too
	if len(fields) != 4 {
		return nil, fail(Malformed, "value has %d colon-separated fields, want 4", len(fields))
	}
	keyset, fp, payloadText := fields[1], fields[2], fields[3]
	if !keysetNameRE.MatchString(keyset) {
		return nil, fail(InvalidKeysetName, "keyset name %s does not match %s", quoteName(keyset), keysetNamePattern)
	}
	if !fingerprintRE.MatchString(fp) {
		return nil, fail(Malformed, "fingerprint field is not 8 lowercase hex characters")
	}
	payload, ok := decodeBase64url(payloadText)
	if !ok {
		return nil, fail(Malformed, "payload is not canonical unpadded base64url")
	}
	if len(payload) < minPayloadLength {
		return nil, fail(Malformed, "payload is %d bytes, want at least %d", len(payload), minPayloadLength)
	}
	if payload[keyLength-1]&0x80 != 0 {
		return nil, fail(Malformed, "ephemeral key has its top bit set")
	}
	return &Ciphertext{
		Keyset:      keyset,
		Fingerprint: fp,
		aad:         []byte(version + ":" + keyset + ":" + fp + ":"),
		ephemeral:   payload[:keyLength],
		sealed:      payload[keyLength:],
	}, nil
}

// ParsePrivateKeyEntry parses "qf1-priv-" + 43 canonical base64url characters
// into the raw (unclamped) 32-byte X25519 scalar (spec 6.1).
func ParsePrivateKeyEntry(s string) ([]byte, error) {
	body, ok := strings.CutPrefix(s, privateKeyPrefix)
	if !ok || len(body) != 43 {
		return nil, fail(InvalidKeyEntry, "not a qf1 private key entry")
	}
	b, ok := decodeBase64url(body)
	if !ok || len(b) != keyLength {
		return nil, fail(InvalidKeyEntry, "not a qf1 private key entry")
	}
	return b, nil
}

type heldKey struct {
	private *ecdh.PrivateKey
	public  []byte
}

// Keyset is a parsed keyset env var: fingerprint -> key (spec 6.2).
type Keyset struct {
	name string
	keys map[string]heldKey
}

// Fingerprints returns the held fingerprints, sorted.
func (k *Keyset) Fingerprints() []string {
	out := make([]string, 0, len(k.keys))
	for fp := range k.keys {
		out = append(out, fp)
	}
	sort.Strings(out)
	return out
}

func fingerprint(pub []byte) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:4])
}

// ParseKeyset parses the value of the env var named name (spec 6.2). set is
// false when the variable is unset.
func ParseKeyset(name, value string, set bool) (*Keyset, error) {
	if !set {
		return nil, fail(KeysetMissing, "this runtime has no %s", name)
	}
	var entries []string
	for _, piece := range strings.Split(value, ",") {
		if piece = strings.Trim(piece, envTrimSet); piece != "" {
			entries = append(entries, piece)
		}
	}
	if len(entries) == 0 {
		return nil, fail(KeysetMissing, "%s is set but holds no keys", name)
	}
	ks := &Keyset{name: name, keys: make(map[string]heldKey, len(entries))}
	for i, entry := range entries {
		raw, err := ParsePrivateKeyEntry(entry)
		if err != nil {
			return nil, fail(KeysetInvalid, "%s entry %d is not a qf1 private key", name, i+1)
		}
		priv, err := ecdh.X25519().NewPrivateKey(raw)
		if err != nil {
			return nil, fail(KeysetInvalid, "%s entry %d is not a qf1 private key", name, i+1)
		}
		pub := priv.PublicKey().Bytes()
		fp := fingerprint(pub)
		if existing, ok := ks.keys[fp]; ok {
			if hmac.Equal(existing.public, pub) {
				continue // the same key twice, or a clamp-equivalent encoding
			}
			return nil, fail(KeysetInvalid, "%s holds two different keys with fingerprint %s", name, fp)
		}
		ks.keys[fp] = heldKey{private: priv, public: pub}
	}
	return ks, nil
}

// x25519 computes X25519(scalar, u). crypto/ecdh clamps the scalar, masks
// nothing (callers reject a set top bit first, 3.5 step 6) and reduces a
// non-canonical u. It returns an error for a small-order u, which is the
// all-zero check of spec 5.2.
func x25519(scalar, u []byte) ([]byte, error) {
	priv, err := ecdh.X25519().NewPrivateKey(scalar)
	if err != nil {
		return nil, err
	}
	return x25519With(priv, u)
}

func x25519With(priv *ecdh.PrivateKey, u []byte) ([]byte, error) {
	pub, err := ecdh.X25519().NewPublicKey(u)
	if err != nil {
		return nil, err
	}
	return priv.ECDH(pub)
}

func hkdfSalt(ephemeral, recipientPublic []byte) []byte {
	salt := make([]byte, 0, len(ephemeral)+len(recipientPublic))
	return append(append(salt, ephemeral...), recipientPublic...)
}

// hkdfExtract and hkdfExpand are RFC 5869 written out over HMAC-SHA256
// (spec 5.3 requires that, and the module's Go floor predates crypto/hkdf).
func hkdfExtract(salt, ikm []byte) []byte {
	m := hmac.New(sha256.New, salt)
	m.Write(ikm)
	return m.Sum(nil)
}

func hkdfExpand(prk, info []byte, length int) []byte {
	var okm, t []byte
	for i := byte(1); len(okm) < length; i++ {
		m := hmac.New(sha256.New, prk)
		m.Write(t)
		m.Write(info)
		m.Write([]byte{i})
		t = m.Sum(nil)
		okm = append(okm, t...)
	}
	return okm[:length]
}

func deriveKeyNonce(shared, ephemeral, recipientPublic []byte) (key, nonce []byte) {
	okm := hkdfExpand(hkdfExtract(hkdfSalt(ephemeral, recipientPublic), shared), hkdfInfo, okmLength)
	return okm[:32], okm[32:]
}

func allZero(b []byte) bool {
	var acc byte
	for _, x := range b {
		acc |= x
	}
	return acc == 0
}

// open decrypts ct with the key this keyset holds for its fingerprint
// (spec 8 steps 3 to 7). There is no trial decryption.
func (k *Keyset) open(ct *Ciphertext) (string, error) {
	held, ok := k.keys[ct.Fingerprint]
	if !ok {
		return "", fail(KeyNotHeld, "value is encrypted to %s fingerprint %s; this runtime holds %s fingerprints [%s]",
			ct.Keyset, ct.Fingerprint, k.name, strings.Join(k.Fingerprints(), " "))
	}
	shared, err := x25519With(held.private, ct.ephemeral)
	if err != nil || allZero(shared) {
		// crypto/ecdh refuses a small-order point with an error: that refusal
		// IS the all-zero failure (spec 5.2).
		return "", fail(InvalidEphemeralKey, "X25519 shared secret for %s fingerprint %s is all zero", ct.Keyset, ct.Fingerprint)
	}
	key, nonce := deriveKeyNonce(shared, ct.ephemeral, held.public)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fail(DecryptFailed, "AES setup failed for %s fingerprint %s", ct.Keyset, ct.Fingerprint)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fail(DecryptFailed, "GCM setup failed for %s fingerprint %s", ct.Keyset, ct.Fingerprint)
	}
	plain, err := gcm.Open(nil, nonce, ct.sealed, ct.aad)
	if err != nil {
		return "", fail(DecryptFailed, "authentication failed for %s fingerprint %s", ct.Keyset, ct.Fingerprint)
	}
	if !utf8.Valid(plain) {
		return "", fail(DecryptFailed, "plaintext for %s fingerprint %s is not valid UTF-8", ct.Keyset, ct.Fingerprint)
	}
	return string(plain), nil
}

// Reader decrypts qf1 values with keysets read lazily from its lookup. A
// parsed keyset is cached per env var and reused while the variable's value
// is unchanged (spec 6.2 / decision 33 allow caching for the process; keying
// on the value also keeps a changed variable honest). Safe for concurrent use.
type Reader struct {
	lookup LookupFunc

	mu    sync.Mutex
	cache map[string]cachedKeyset
}

type cachedKeyset struct {
	value  string
	keyset *Keyset
	err    error
}

// NewReader returns a Reader that reads keyset env vars through lookup.
func NewReader(lookup LookupFunc) *Reader {
	return &Reader{lookup: lookup, cache: make(map[string]cachedKeyset)}
}

func (r *Reader) keyset(name string) (*Keyset, error) {
	value, set := r.lookup(name)
	if !set {
		return ParseKeyset(name, "", false)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.cache[name]; ok && c.value == value {
		return c.keyset, c.err
	}
	ks, err := ParseKeyset(name, value, true)
	r.cache[name] = cachedKeyset{value: value, keyset: ks, err: err}
	return ks, err
}

// Decrypt reads one stored qf1 value (spec 8). Only the env var named by the
// value's keyset field is read, and only after that name passes spec 2.
func (r *Reader) Decrypt(v string) (string, error) {
	ct, err := Parse(v)
	if err != nil {
		return "", err
	}
	ks, err := r.keyset(ct.Keyset)
	if err != nil {
		if qe, ok := err.(*Error); ok && qe.Class == KeysetMissing {
			return "", fail(KeysetMissing, "value is encrypted to %s fingerprint %s; %s", ct.Keyset, ct.Fingerprint, qe.detail)
		}
		return "", err
	}
	return ks.open(ct)
}
