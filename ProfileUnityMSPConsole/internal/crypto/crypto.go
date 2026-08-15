// Package crypto encrypts tenant credentials at rest with a key held
// outside the database (project brief §9). Losing the key means losing
// the ability to decrypt stored credentials, by design -- the key is
// never stored alongside the data it protects.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
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

// EnsureKey makes sure an encryption key exists at path, generating a
// new random one (base64-encoded, like the PUMC_CREDENTIAL_ENCRYPTION_KEY
// env var format) if the file isn't present yet, and reusing it as-is on
// every later call -- mirroring internal/tlscert.EnsureSelfSigned's
// generate-once-then-reuse pattern. It reports whether it generated a
// new key. Never overwrites an existing file: losing or changing this
// key makes every credential encrypted under it permanently
// undecryptable, so a corrupt or wrong-length existing file is a hard
// error rather than something to silently regenerate over.
func EnsureKey(path string) (key []byte, generated bool, err error) {
	if _, statErr := os.Stat(path); statErr == nil {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, false, fmt.Errorf("crypto: read key file %s: %w", path, err)
		}
		key, err := base64.StdEncoding.DecodeString(string(raw))
		if err != nil {
			return nil, false, fmt.Errorf("crypto: key file %s is not valid base64: %w", path, err)
		}
		if len(key) != KeySize {
			return nil, false, fmt.Errorf("crypto: key file %s: %w (got %d bytes)", path, ErrInvalidKeySize, len(key))
		}
		return key, false, nil
	}

	key = make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, false, fmt.Errorf("crypto: generate key: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		return nil, false, fmt.Errorf("crypto: write key file %s: %w", path, err)
	}
	return key, true, nil
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
