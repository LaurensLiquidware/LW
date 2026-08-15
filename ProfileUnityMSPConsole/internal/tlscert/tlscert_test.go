package tlscert

import (
	"crypto/tls"
	"path/filepath"
	"testing"
)

func TestEnsureSelfSigned_GeneratesUsableCert(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	generated, err := EnsureSelfSigned(certPath, keyPath, []string{"localhost", "127.0.0.1"})
	if err != nil {
		t.Fatalf("EnsureSelfSigned: %v", err)
	}
	if !generated {
		t.Error("expected generated=true on first call")
	}

	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		t.Fatalf("generated cert/key are not a valid pair: %v", err)
	}
}

func TestEnsureSelfSigned_DoesNotOverwriteExisting(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	if _, err := EnsureSelfSigned(certPath, keyPath, []string{"localhost"}); err != nil {
		t.Fatal(err)
	}
	firstPair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}

	generated, err := EnsureSelfSigned(certPath, keyPath, []string{"localhost"})
	if err != nil {
		t.Fatalf("second EnsureSelfSigned: %v", err)
	}
	if generated {
		t.Error("expected generated=false when cert/key already exist")
	}

	secondPair, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstPair.Certificate[0]) != string(secondPair.Certificate[0]) {
		t.Error("certificate was regenerated instead of left alone")
	}
}
