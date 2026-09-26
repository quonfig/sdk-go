package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"strings"
	"testing"
)

const testKey = "e657e0406fc22e17d3145966396b2130d33dcb30ac0edd62a77235cdd01fc49d"

// encryptForTest produces DATA--IV--AUTH_TAG the way the server does.
func encryptForTest(t *testing.T, keyHex, plaintext string) string {
	t.Helper()
	key, _ := hex.DecodeString(keyHex)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	iv := []byte("0123456789ab")
	sealed := gcm.Seal(nil, iv, []byte(plaintext), nil)
	data, tag := sealed[:len(sealed)-gcm.Overhead()], sealed[len(sealed)-gcm.Overhead():]
	return hex.EncodeToString(data) + "--" + hex.EncodeToString(iv) + "--" + hex.EncodeToString(tag)
}

func TestDecryptValue_RoundTrip(t *testing.T) {
	enc := encryptForTest(t, testKey, "hello secret")
	got, err := DecryptValue(testKey, enc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello secret" {
		t.Fatalf("got %q", got)
	}
}

// Malformed ciphertext must return an error, never panic (qfg-9dxb.4). Before
// the fix, a data+tag shorter than the 16-byte GCM tag sliced out of range.
func TestDecryptValue_MalformedReturnsErrorNotPanic(t *testing.T) {
	valid := encryptForTest(t, testKey, "x")
	parts := strings.Split(valid, "--")
	cases := map[string]string{
		"short data+tag (audit repro)": "AA--00112233445566778899AABB--BB",
		"empty data and tag":           "--00112233445566778899AABB--",
		"tag one byte short":           "--" + parts[1] + "--" + parts[2][:30],
		"empty IV":                     parts[0] + "----" + parts[2],
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("DecryptValue panicked: %v", r)
				}
			}()
			if _, err := DecryptValue(testKey, value); err == nil {
				t.Fatalf("expected an error for %q", value)
			}
		})
	}
}
