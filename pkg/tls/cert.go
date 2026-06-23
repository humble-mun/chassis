package tls

import (
	cryptotls "crypto/tls"
	"fmt"

	"github.com/go-logr/logr"
)

// CertReloader serves a server certificate that is reloaded from disk whenever
// the certificate or key file is rotated in place. Use GetCertificate as the
// crypto/tls.Config.GetCertificate callback so renewals take effect on the next
// handshake without restarting the server.
type CertReloader struct {
	certPath, keyPath string
	inner             *reloader[cryptotls.Certificate]
}

// NewCertReloader loads the certificate/key pair from the given paths and
// returns a reloader. The initial load is performed eagerly so a missing or
// invalid pair fails fast.
func NewCertReloader(logger logr.Logger, certPath, keyPath string) (*CertReloader, error) {
	cr := &CertReloader{certPath: certPath, keyPath: keyPath}
	inner, err := newReloader(
		logger,
		"TLS certificate",
		[]string{certPath, keyPath},
		cr.load,
	)
	if err != nil {
		return nil, err
	}
	cr.inner = inner
	return cr, nil
}

// load reads the current certificate/key pair from disk.
func (r *CertReloader) load() (*cryptotls.Certificate, error) {
	cert, err := cryptotls.LoadX509KeyPair(r.certPath, r.keyPath)
	if err != nil {
		return nil, fmt.Errorf("load X509 key pair (%q, %q): %w", r.certPath, r.keyPath, err)
	}
	return &cert, nil
}

// GetCertificate returns the current certificate, reloading it first if the
// underlying files have rotated. It satisfies crypto/tls.Config.GetCertificate.
// On reload failure it logs and serves the previously loaded certificate.
func (r *CertReloader) GetCertificate(*cryptotls.ClientHelloInfo) (*cryptotls.Certificate, error) {
	return r.inner.current(), nil
}
