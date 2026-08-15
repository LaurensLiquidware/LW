package tlscert

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
)

func genCertPEM(t *testing.T, hosts []string) (certPEM, keyPEM []byte) {
	t.Helper()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if _, err := EnsureSelfSigned(certPath, keyPath, hosts); err != nil {
		t.Fatalf("EnsureSelfSigned: %v", err)
	}
	cert, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}

func TestHolder_GetCertificate_ErrorsBeforeSet(t *testing.T) {
	h := NewHolder()
	if _, err := h.GetCertificate(nil); err == nil {
		t.Fatal("expected an error from GetCertificate before Set is ever called")
	}
}

func TestHolder_Set_RejectsMismatchedPair(t *testing.T) {
	h := NewHolder()
	certA, _ := genCertPEM(t, []string{"a.example"})
	_, keyB := genCertPEM(t, []string{"b.example"})

	if err := h.Set(certA, keyB); err == nil {
		t.Fatal("expected an error for a certificate/key pair that don't match")
	}
}

// TestHolder_HotSwap_RealTLSHandshakePicksUpTheNewCert proves the swap
// works against a real TLS handshake over a real TCP connection, not
// just against the Holder's own in-memory state.
func TestHolder_HotSwap_RealTLSHandshakePicksUpTheNewCert(t *testing.T) {
	certA, keyA := genCertPEM(t, []string{"127.0.0.1"})
	certB, keyB := genCertPEM(t, []string{"127.0.0.1"})

	h := NewHolder()
	if err := h.Set(certA, keyA); err != nil {
		t.Fatalf("Set(A): %v", err)
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{GetCertificate: h.GetCertificate})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	acceptOnce := func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.(*tls.Conn).Handshake()
	}

	dialAndGetCert := func() []byte {
		go acceptOnce()
		conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer conn.Close()
		certs := conn.ConnectionState().PeerCertificates
		if len(certs) == 0 {
			t.Fatal("no peer certificate presented")
		}
		return certs[0].Raw
	}

	firstSeen := dialAndGetCert()

	if err := h.Set(certB, keyB); err != nil {
		t.Fatalf("Set(B): %v", err)
	}
	secondSeen := dialAndGetCert()

	if string(firstSeen) == string(secondSeen) {
		t.Error("second handshake presented the same certificate as the first -- the hot swap never took effect")
	}
}
