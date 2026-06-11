package server

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/humble-mun/chassis/pkg/service"
)

const (
	flagHTTPBindAddress = "http-bind-address"
	flagTLSCertPath     = "tls-cert-path"
	flagTLSKeyPath      = "tls-key-path"
)

// RegisterFlags registers HTTP server configuration flags
func RegisterFlags(pfs *pflag.FlagSet) {
	pfs.String(flagHTTPBindAddress, service.DefaultHTTPServerBind, "The address to bind the HTTP server.")
	pfs.String(flagTLSCertPath, service.DefaultTLSCertPath, "The path to the TLS certificate file.")
	pfs.String(flagTLSKeyPath, service.DefaultTLSKeyPath, "The path to the TLS key file.")
}

// WithDefaultListener builds a listener from the registered flags
// (http-bind-address, tls-cert-path, tls-key-path) and appends it to the
// server options. It is a no-op when no flags have been bound via RegisterFlags.
// BaseContext calls this automatically so that the flag-driven listener is
// always present unless the caller supplies explicit listeners instead.
func WithDefaultListener() Option {
	return func(o *options) {
		o.listeners = append(o.listeners, listenerConfig{
			addrFn:     func() string { return viper.GetString(flagHTTPBindAddress) },
			tlsCertFn:  func() string { return viper.GetString(flagTLSCertPath) },
			tlsKeyFn:   func() string { return viper.GetString(flagTLSKeyPath) },
		})
	}
}

// WithDefaultCORSConfig applies a permissive CORS policy that allows all
// origins, credentials, and a standard set of request headers. It is the
// recommended starting point for services that need CORS support.
// Additional CORSOption values can be passed to override individual fields
// of the default configuration before it is applied.
func WithDefaultCORSConfig(opts ...CORSOption) Option {
	return func(o *options) {
		cfg := cors.DefaultConfig()
		cfg.AllowAllOrigins = true
		cfg.AllowCredentials = true
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
		logger:    logger.WithName("http"),
		engine:    gin.New(),
		grpc:      o.grpc,
		listeners: o.listeners,
	}
	middleware := []gin.HandlerFunc{ginLogger(logger), gin.Recovery()}
	if o.corsConfig != nil {
		middleware = append([]gin.HandlerFunc{cors.New(*o.corsConfig)}, middleware...)
	}
	server.engine.Use(middleware...)
	return server
}
