package server

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/humble-mun/chassis/pkg/constants"
)

const (
	flagHTTPBindAddress = "http-bind-address"
	flagTLSCertPath     = "tls-cert-path"
	flagTLSKeyPath      = "tls-key-path"
)

// RegisterFlags registers HTTP server configuration flags
func RegisterFlags(pfs *pflag.FlagSet) {
	pfs.String(flagHTTPBindAddress, constants.DefaultHTTPServerBind, "The address to bind the HTTP server.")
	pfs.String(flagTLSCertPath, constants.DefaultTLSCertPath, "The path to the TLS certificate file.")
	pfs.String(flagTLSKeyPath, constants.DefaultTLSKeyPath, "The path to the TLS key file.")
}

// WithDefaultListener builds a listener from the registered flags
// (http-bind-address, tls-cert-path, tls-key-path) and appends it to the
// server options. Values are resolved lazily at Start time so that flag and
// config file values are available. BaseContext calls this automatically so
// that the flag-driven listener is always present unless the caller supplies
// explicit listeners instead.
func WithDefaultListener() Option {
	return func(o *options) {
		o.listeners = append(o.listeners, listenerConfig{
			addrFn:    func() string { return viper.GetString(flagHTTPBindAddress) },
			tlsCertFn: func() string { return viper.GetString(flagTLSCertPath) },
			tlsKeyFn:  func() string { return viper.GetString(flagTLSKeyPath) },
		})
	}
}

// WithDefaultCORSConfig applies a baseline CORS policy with a standard set of
// request headers. By default it keeps cors.DefaultConfig() semantics, which
// disallow wildcard origins and credentials; this is the safe default for
// production. Additional CORSOption values can be passed to override individual
// fields, for example WithCORSAllowAllOrigins for development and debugging.
func WithDefaultCORSConfig(opts ...CORSOption) Option {
	return func(o *options) {
		cfg := cors.DefaultConfig()
		cfg.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
		for _, opt := range opts {
			opt(&cfg)
		}
		o.corsConfig = &cfg
	}
}

// NewHTTPServer creates a new HTTP server with Gin and optional gRPC support.
// When no WithListeners option is provided, Start falls back to a single
// listener derived from the registered flags via WithDefaultListener.
func NewHTTPServer(logger logr.Logger, opts ...Option) *HTTPServer {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	server := &HTTPServer{
		logger:            logger.WithName("http"),
		engine:            gin.New(),
		grpc:              o.grpc,
		listeners:         o.listeners,
		readHeaderTimeout: o.readHeaderTimeout,
	}
	middleware := []gin.HandlerFunc{ginLogger(logger), gin.Recovery()}
	if o.corsConfig != nil && corsAllowsAnyOrigin(o.corsConfig) {
		middleware = append([]gin.HandlerFunc{cors.New(*o.corsConfig)}, middleware...)
	}
	server.engine.Use(middleware...)
	return server
}

// corsAllowsAnyOrigin reports whether cfg permits at least one cross-origin
// request. The bare cors.DefaultConfig() permits none, which cors.New rejects
// with a panic; in that case the CORS middleware is simply omitted, leaving the
// secure same-origin default intact instead of failing server construction.
func corsAllowsAnyOrigin(cfg *cors.Config) bool {
	return cfg.AllowAllOrigins ||
		cfg.AllowOriginFunc != nil ||
		cfg.AllowOriginWithContextFunc != nil ||
		len(cfg.AllowOrigins) > 0
}
