package app

import (
	"google.golang.org/grpc"

	"github.com/humble-mun/chassis/pkg/server"
)

// Option configures the BaseContext bootstrap behavior.
type Option func(*options)

type options struct {
	init          func() error
	serverOptions []server.Option
}

// WithInit sets the viper initialization function produced by PrepareFlags.
func WithInit(init func() error) Option {
	return func(o *options) {
		o.init = init
	}
}

// WithGRPCServer attaches a gRPC server to the shared H2C listener.
// When set, inbound requests with Content-Type: application/grpc are
// routed to the gRPC server; all other requests go to Gin.
func WithGRPCServer(s *grpc.Server) Option {
	return func(o *options) {
		o.serverOptions = append(o.serverOptions, server.WithGRPCServer(s))
	}
}

// WithTCPListener appends a TCP listener to the HTTP server.
// Use server.WithAddr to supply the bind address and server.WithTLSCert to
// enable TLS. server.WithAddr must be included; Start returns an error if the
// address is empty when the server is started.
func WithTCPListener(opts ...server.ListenerOption) Option {
	return func(o *options) {
		o.serverOptions = append(o.serverOptions, server.WithTCPListener(opts...))
	}
}

// WithUnixListener appends a Unix domain socket listener to the HTTP server.
// Use server.WithAddr to supply the socket path and server.WithTLSCert to
// enable TLS. server.WithAddr must be included; Start returns an error if the
// path is empty when the server is started.
func WithUnixListener(opts ...server.ListenerOption) Option {
	return func(o *options) {
		o.serverOptions = append(o.serverOptions, server.WithUnixListener(opts...))
	}
}
