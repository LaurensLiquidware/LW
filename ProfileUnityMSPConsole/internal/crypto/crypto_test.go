package crypto

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func testKey() []byte {
	key := make([]byte, KeySize)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := testKey()
	plaintext := "s3cr3t-password-with-&-and-=-chars"

	blob, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Contains(blob, []byte(plaintext)) {
		t.Fatal("ciphertext must not contain the plaintext")
	}

	got, err := Decrypt(key, blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if got != plaintext {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestDecrypt_WrongKeyFails(t *testing.T) {
	key := testKey()
	wrongKey := make([]byte, KeySize)
	copy(wrongKey, key)
	wrongKey[0] ^= 0xFF

	blob, err := Encrypt(key, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(wrongKey, blob); err == nil {
		t.Fatal("expected decryption to fail with the wrong key")
	}
}

func TestEncrypt_RejectsWrongKeySize(t *testing.T) {
	if _, err := Encrypt([]byte("too-short"), "x"); err != ErrInvalidKeySize {
		t.Errorf("err = %v, want ErrInvalidKeySize", err)
	}
}

func TestDecrypt_RejectsTruncatedBlob(t *testing.T) {
	key := testKey()
	if _, err := Decrypt(key, []byte("short")); err == nil {
		t.Fatal("expected an error for a truncated blob")
	}
}

func TestEnsureKey_GeneratesUsableKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")

	key, generated, err := EnsureKey(path)
	if err != nil {
		t.Fatalf("EnsureKey: %v", err)
	}
	if !generated {
		t.Error("expected generated=true on first call")
	}
	if len(key) != KeySize {
		t.Fatalf("key length = %d, want %d", len(key), KeySize)
	}

	blob, err := Encrypt(key, "round trip")
	if err != nil {
		t.Fatalf("Encrypt with generated key: %v", err)
	}
	if plaintext, err := Decrypt(key, blob); err != nil || plaintext != "round trip" {
		t.Fatalf("Decrypt with generated key: got (%q, %v)", plaintext, err)
	}
}

func TestEnsureKey_DoesNotOverwriteExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")

	firstKey, generated, err := EnsureKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !generated {
		t.Fatal("expected generated=true on first call")
	}

	secondKey, generated, err := EnsureKey(path)
	if err != nil {
		t.Fatalf("second EnsureKey: %v", err)
	}
	if generated {
		t.Error("expected generated=false when the key file already exists")
	}
	if !bytes.Equal(firstKey, secondKey) {
		t.Error("key was regenerated instead of left alone")
	}
}

func TestEnsureKey_RejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte("not valid base64!!"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := EnsureKey(path); err == nil {
		t.Fatal("expected an error for a corrupt key file")
	}
}

func TestEnsureKey_RejectsWrongLengthKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if err := os.WriteFile(path, []byte(short), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := EnsureKey(path); err == nil {
		t.Fatal("expected an error for a wrong-length key file")
	}
}
