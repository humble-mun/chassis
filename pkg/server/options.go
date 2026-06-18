package server

import (
	"errors"
	"time"

	"github.com/gin-contrib/cors"
	"google.golang.org/grpc"
)

// ErrAddrMissing is returned by Start when a listener was registered
// without a bind address or socket path. Use WithAddr to supply the address
// as a lazy-evaluated option so that flag values are resolved at start time.
var ErrAddrMissing = errors.New("listener addr missing")

// listenerConfig describes a single listen endpoint.
type listenerConfig struct {
	addrFn        func() string
	network       string // "tcp" (default) or "unix"
	tlsCertFn     func() string
	tlsKeyFn      func() string
	clientCAFn    func() string
	tlsMinVersion uint16 // 0 means use the crypto/tls default
}

// ListenerOption configures a single listener endpoint.
type ListenerOption func(*listenerConfig)

// WithAddr sets the listen address (or Unix socket path) provider. fn is
// stored and invoked lazily at Start time, allowing the value to be sourced
// from a flag or other late-bound configuration that is not yet populated
// when the Option is applied. WithAddr must be supplied to WithTCPListener
// or WithUnixListener; omitting it leaves the address empty, which Start will
// reject with an error.
func WithAddr(fn func() string) ListenerOption {
	return func(lc *listenerConfig) {
		lc.addrFn = fn
	}
}

// WithTLSCert enables TLS on the listener using the certificate and key paths
// returned by certFn and keyFn. Both providers are invoked lazily at Start
// time; when either resolves to an empty string the listener stays in
// plain-text mode.
func WithTLSCert(certFn, keyFn func() string) ListenerOption {
	return func(lc *listenerConfig) {
		lc.tlsCertFn = certFn
		lc.tlsKeyFn = keyFn
	}
}

// WithMTLS enables mutual TLS on the listener, verifying client certificates
// against the CA bundle whose path clientCAFn returns. clientCAFn is invoked
// lazily at Start time. The server certificate and key must also be provided
// via WithTLSCert; mTLS has no effect on a plaintext listener.
func WithMTLS(clientCAFn func() string) ListenerOption {
	return func(lc *listenerConfig) {
		lc.clientCAFn = clientCAFn
	}
}

// WithTLSMinVersion sets the minimum accepted TLS version on the listener,
// using the crypto/tls version constants (for example tls.VersionTLS12 or
// tls.VersionTLS13). It has no effect on a plaintext listener. When omitted,
// the crypto/tls default minimum version applies, except on mTLS listeners
// which always negotiate TLS 1.3 or higher.
func WithTLSMinVersion(version uint16) ListenerOption {
	return func(lc *listenerConfig) {
		lc.tlsMinVersion = version
	}
}

// Option configures an HTTP server.
type Option func(*options)

type options struct {
	grpc              *grpc.Server
	listeners         []listenerConfig
	corsConfig        *cors.Config
	readHeaderTimeout time.Duration
}

// CORSOption adjusts a cors.Config before it is applied to the server.
// Use with WithDefaultCORSConfig to override individual fields of the
// default policy.
type CORSOption func(*cors.Config)

// WithCORSAllowAllOrigins relaxes the CORS policy to accept any origin and to
// allow credentials. It is intended for development and debugging only; do not
// enable it in production, where it would let any site make credentialed
// cross-origin requests.
func WithCORSAllowAllOrigins() CORSOption {
	return func(cfg *cors.Config) {
		cfg.AllowAllOrigins = true
		cfg.AllowCredentials = true
	}
}

// WithReadHeaderTimeout sets the http.Server ReadHeaderTimeout for every
// listener. It bounds how long the server waits to read request headers,
// mitigating slowloris-style attacks. When omitted (the zero value), Start
// applies a default of 5 seconds.
func WithReadHeaderTimeout(d time.Duration) Option {
	return func(o *options) {
		o.readHeaderTimeout = d
	}
}

// WithGRPCServer attaches a gRPC server to the shared H2C handler.
// Inbound requests whose Content-Type starts with "application/grpc"
// are routed to the gRPC server; all other requests go to Gin.
func WithGRPCServer(s *grpc.Server) Option {
	return func(o *options) {
		o.grpc = s
	}
}

// WithTCPListener appends a TCP listener. Use WithAddr to specify the bind
// address and WithTLSCert to enable TLS. WithAddr must be provided; Start
// returns an error if the address is empty when the server is started.
func WithTCPListener(opts ...ListenerOption) Option {
	return func(o *options) {
		lc := listenerConfig{network: "tcp"}
		for _, opt := range opts {
			opt(&lc)
		}
		o.listeners = append(o.listeners, lc)
	}
}

// WithUnixListener appends a Unix domain socket listener. Use WithAddr to
// specify the socket path and WithTLSCert to enable TLS. WithAddr must be
// provided; Start returns an error if the path is empty when the server is
// started.
func WithUnixListener(opts ...ListenerOption) Option {
	return func(o *options) {
		lc := listenerConfig{network: "unix"}
		for _, opt := range opts {
			opt(&lc)
		}
		o.listeners = append(o.listeners, lc)
	}
}
