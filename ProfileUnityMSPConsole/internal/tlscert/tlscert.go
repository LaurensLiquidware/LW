// Package tlscert generates a self-signed TLS certificate at first setup
// (project brief §9, carried over from the reference project) when no
// real certificate has been supplied. It never overwrites a certificate
// that already exists — an operator's own CA-signed cert is left alone.
package tlscert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// EnsureSelfSigned makes sure a certificate and key exist at certPath and
// keyPath, generating a self-signed one covering hosts if neither file is
// present yet. It reports whether it generated a new pair.
func EnsureSelfSigned(certPath, keyPath string, hosts []string) (generated bool, err error) {
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)
	if certErr == nil && keyErr == nil {
		return false, nil
	}
	if certErr == nil || keyErr == nil {
		return false, fmt.Errorf("tlscert: only one of cert/key exists (%s, %s) — remove the stale one or supply both", certPath, keyPath)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return false, fmt.Errorf("tlscert: generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return false, fmt.Errorf("tlscert: generate serial: %w", err)
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{Organization: []string{"ProfileUnity MSP Licensing Console (self-signed)"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(2, 0, 0), // 2 years
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else if h != "" {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return false, fmt.Errorf("tlscert: create certificate: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		return false, fmt.Errorf("tlscert: create cert directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o755); err != nil {
		return false, fmt.Errorf("tlscert: create key directory: %w", err)
	}

	if err := writePEM(certPath, "CERTIFICATE", certDER, 0o644); err != nil {
		return false, err
	}
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return false, fmt.Errorf("tlscert: marshal key: %w", err)
	}
	if err := writePEM(keyPath, "EC PRIVATE KEY", keyBytes, 0o600); err != nil {
		return false, err
	}

	return true, nil
}

func writePEM(path, blockType string, der []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("tlscert: open %s: %w", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		return fmt.Errorf("tlscert: write %s: %w", path, err)
	}
	return nil
}
