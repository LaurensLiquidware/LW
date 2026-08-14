// Package crypto encrypts tenant credentials at rest with a key held
// outside the database (project brief §9). Losing the key means losing
// the ability to decrypt stored credentials, by design -- the key is
// never stored alongside the data it protects.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// KeySize is the required key length for AES-256-GCM.
const KeySize = 32

// ErrInvalidKeySize is returned when a key is not exactly KeySize bytes.
var ErrInvalidKeySize = fmt.Errorf("crypto: key must be %d bytes", KeySize)

// Encrypt seals plaintext with AES-256-GCM under key, returning a single
// blob (nonce prepended to the ciphertext) suitable for storing as-is.
func Encrypt(key []byte, plaintext string) ([]byte, error) {
	if len(key) != KeySize {
		return nil, ErrInvalidKeySize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: generate nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

// Decrypt reverses Encrypt. It fails closed: a wrong key, truncated blob,
// or tampered ciphertext all return an error rather than garbage text.
func Decrypt(key []byte, blob []byte) (string, error) {
	if len(key) != KeySize {
		return "", ErrInvalidKeySize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new GCM: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(blob) < nonceSize {
		return "", errors.New("crypto: ciphertext too short")
	}
	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt: %w", err)
	}
	return string(plaintext), nil
}
