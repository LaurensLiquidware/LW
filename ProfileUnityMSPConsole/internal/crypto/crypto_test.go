package crypto

import (
	"bytes"
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
