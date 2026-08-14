package tlscert

import (
	"crypto/tls"
	"fmt"
	"sync/atomic"
)

// Holder makes the TLS certificate an HTTPS listener uses swappable at
// runtime without a restart: an operator can upload a real certificate
// from the Settings screen and every TLS handshake from that point on
// uses it, while any handshake already in flight is unaffected.
//
// Wire it in via GetCertificate on an *tls.Config passed to
// http.Server.TLSConfig, and call server.ListenAndServeTLS("", "") (both
// arguments empty) so the standard library defers to GetCertificate
// instead of trying to load a cert from disk itself.
type Holder struct {
	cert atomic.Pointer[tls.Certificate]
}

// NewHolder creates an empty Holder. Call Set before starting the
// listener — GetCertificate returns an error until then.
func NewHolder() *Holder {
	return &Holder{}
}

// Set validates certPEM/keyPEM as a matching pair and, only if they're
// valid, makes them the active certificate for every future handshake.
func (h *Holder) Set(certPEM, keyPEM []byte) error {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("tlscert: parse certificate/key pair: %w", err)
	}
	h.cert.Store(&cert)
	return nil
}

// GetCertificate is a tls.Config.GetCertificate callback returning
// whatever certificate is currently active. There is only ever one
// certificate for this server, so the ClientHelloInfo (usually used for
// SNI-based selection among several certs) is ignored.
func (h *Holder) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert := h.cert.Load()
	if cert == nil {
		return nil, fmt.Errorf("tlscert: no certificate loaded yet")
	}
	return cert, nil
}
