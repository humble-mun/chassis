package tls

import (
	cryptotls "crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/go-logr/logr"
)

// CAReloader serves an x509.CertPool that is reloaded from disk whenever the
// client-CA bundle file is rotated in place. Use CurrentPool to seed
// tls.Config.ClientCAs and GetConfigForClient (see ConfigForClient) so CA
// rotations take effect on the next handshake without restarting the server.
type CAReloader struct {
	caPath string
	inner  *reloader[x509.CertPool]
}

// NewCAReloader loads the client-CA bundle from caPath and returns a reloader.
// The initial load is performed eagerly so a missing or invalid bundle fails
// fast.
func NewCAReloader(logger logr.Logger, caPath string) (*CAReloader, error) {
	cr := &CAReloader{caPath: caPath}
	inner, err := newReloader(
		logger,
		"client CA",
		[]string{caPath},
		cr.load,
	)
	if err != nil {
		return nil, err
	}
	cr.inner = inner
	return cr, nil
}

// load reads the current client-CA bundle from disk into a fresh pool.
func (r *CAReloader) load() (*x509.CertPool, error) {
	pem, err := os.ReadFile(r.caPath)
	if err != nil {
		return nil, fmt.Errorf("read client CA %q: %w", r.caPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("parse client CA from %q", r.caPath)
	}
	return pool, nil
}

// CurrentPool returns the current CA pool, reloading it first if the underlying
// file has rotated. On reload failure it logs and serves the previously loaded
// pool.
func (r *CAReloader) CurrentPool() *x509.CertPool {
	return r.inner.current()
}

// ConfigForClient returns a crypto/tls.Config.GetConfigForClient callback that
// stamps the current CA pool onto a clone of base for every incoming
// connection. base is cloned (not mutated) so the per-connection config is
// independent of later rotations.
func (r *CAReloader) ConfigForClient(base *cryptotls.Config) func(*cryptotls.ClientHelloInfo) (*cryptotls.Config, error) {
	return func(*cryptotls.ClientHelloInfo) (*cryptotls.Config, error) {
		c := base.Clone()
		c.ClientCAs = r.CurrentPool()
		return c, nil
	}
}
