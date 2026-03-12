package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrInvalidValueFormat is returned when the encrypted value does not have the expected DATA--IV--AUTH_TAG format.
var ErrInvalidValueFormat = errors.New("invalid encrypted value format: expected DATA--IV--AUTH_TAG")

const validPartCount = 3

// DecryptValue decrypts an AES-GCM encrypted value.
// The secretKeyString is a hex-encoded AES key.
// The value is formatted as "DATA--IV--AUTH_TAG" where each part is hex-encoded.
func DecryptValue(secretKeyString string, value string) (string, error) {
	// Decode the hex-encoded secret key
	secretKey, err := hex.DecodeString(strings.ToUpper(secretKeyString))
	if err != nil {
		return "", err
	}

	// Split the value into data, IV, and auth tag parts
	parts := strings.SplitN(strings.ToUpper(value), "--", validPartCount)
	if len(parts) < validPartCount {
		return "", ErrInvalidValueFormat
	}

	dataStr, ivStr, authTagStr := parts[0], parts[1], parts[2]

	// Decode the hex-encoded parts
	iv, err := hex.DecodeString(ivStr)
	if err != nil {
		return "", err
	}

	dataToProcess, err := hex.DecodeString(dataStr + authTagStr)
	if err != nil {
		return "", err
	}

	// Initialize AES block cipher
	block, err := aes.NewCipher(secretKey)
	if err != nil {
		return "", err
	}

	// Initialize GCM with the nonce size matching the IV length
	gcm, err := cipher.NewGCMWithNonceSize(block, len(iv))
	if err != nil {
		return "", err
	}

	// The ciphertext for gcm.Open is data + authTag
	data := dataToProcess[:len(dataToProcess)-gcm.Overhead()]
	authTag := dataToProcess[len(dataToProcess)-gcm.Overhead():]

	// Decrypt the data
	decryptedData, err := gcm.Open(nil, iv, append(data, authTag...), nil)
	if err != nil {
		return "", err
	}

	return string(decryptedData), nil
}
