package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/humble-mun/chassis/pkg/metrics"
	tlsreload "github.com/humble-mun/chassis/pkg/tls"
	"github.com/humble-mun/chassis/pkg/utils"
	"github.com/humble-mun/chassis/pkg/version"
)

// HTTPServer is an HTTP server implementation using the HTTPServer framework
type HTTPServer struct {
	logger            logr.Logger
	engine            *gin.Engine
	grpc              *grpc.Server
	listeners         []listenerConfig
	readHeaderTimeout time.Duration
}

// RegisterRoute registers custom routes with the Gin engine
func (h HTTPServer) RegisterRoute(api func(engine *gin.Engine)) {
	api(h.engine)
}

// resolvedListener holds a listenerConfig with all lazy providers evaluated
// into concrete strings. Providers are resolved exactly once at Start time,
// after the viper configuration loader has populated flag values.
type resolvedListener struct {
	addr          string
	network       string
	tlsCertPath   string
	tlsKeyPath    string
	clientCAPath  string
	tlsMinVersion uint16
}

// resolve evaluates lc's lazy providers into a resolvedListener. A nil
// provider yields an empty string, preserving the plain-text / no-mTLS
// behavior of an unset option.
func resolve(lc listenerConfig) resolvedListener {
	call := func(fn func() string) string {
		if fn == nil {
			return ""
		}
		return fn()
	}
	return resolvedListener{
		addr:          call(lc.addrFn),
		network:       lc.network,
		tlsCertPath:   call(lc.tlsCertFn),
		tlsKeyPath:    call(lc.tlsKeyFn),
		clientCAPath:  call(lc.clientCAFn),
		tlsMinVersion: lc.tlsMinVersion,
	}
}

// Start starts the HTTP server(s) with graceful shutdown support.
// When no listeners were provided at construction time, a single default
// listener is derived from the registered flags via WithDefaultListener.
func (h HTTPServer) Start(ctx context.Context) (err error) {
	listeners := h.listeners
	if len(listeners) == 0 {
		o := &options{}
		WithDefaultListener()(o)
		listeners = o.listeners
	}

	readHeaderTimeout := h.readHeaderTimeout
	if readHeaderTimeout == 0 {
		readHeaderTimeout = 5 * time.Second
	}

	// Resolve every listener's lazy providers exactly once, now that viper
	// configuration has been loaded.
	resolved := make([]resolvedListener, len(listeners))
	for i, lc := range listeners {
		resolved[i] = resolve(lc)
	}

	ginHandler := h.engine.Handler()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.grpc != nil && r.ProtoMajor == 2 && strings.HasPrefix(
			r.Header.Get("Content-Type"), "application/grpc") {
			h.grpc.ServeHTTP(w, r)
			return
		}
		ginHandler.ServeHTTP(w, r)
	})

	servers := make([]*http.Server, len(resolved))
	for i, rl := range resolved {
		if rl.addr == "" {
			err = fmt.Errorf("listener %d (%s): %w", i, networkOf(rl), ErrAddrMissing)
			return
		}
		srv := &http.Server{Handler: handler, ReadHeaderTimeout: readHeaderTimeout}
		p := new(http.Protocols)
		if rl.tlsCertPath != "" && rl.tlsKeyPath != "" {
			// TLS listener: HTTP/1.1 + HTTP/2 over TLS (standard ALPN negotiation).
			p.SetHTTP1(true)
			p.SetHTTP2(true)
		} else {
			// Plain listener: HTTP/1.1 + unencrypted HTTP/2 (h2c).
			p.SetHTTP1(true)
			p.SetUnencryptedHTTP2(true)
		}
		srv.Protocols = p
		servers[i] = srv
	}

	group, groupCtx := errgroup.WithContext(ctx)

	// Start a listener goroutine per resolved listener.
	for i, rl := range resolved {
		srv := servers[i]
		group.Go(func() error {
			return h.serveOne(srv, rl)
		})
	}

	// Shutdown all servers when ctx is cancelled.
	group.Go(func() error {
		<-groupCtx.Done()
		sc, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for i, srv := range servers {
			rl := resolved[i]
			logger := h.logger.WithValues("addr", rl.addr, "network", networkOf(rl))
			if shutErr := srv.Shutdown(sc); shutErr != nil {
				logger.Error(shutErr, "shutdown failed")
			} else {
				logger.Info("shutdown succeeded")
			}
		}
		return nil
	})

	err = group.Wait()
	return
}

// NeedLeaderElection indicates whether this server requires leader election
func (h HTTPServer) NeedLeaderElection() bool {
	return false
}

// serveOne opens a net.Listener for rl, wraps it in TLS when configured, and
// runs srv until it is shut down. It returns nil on clean shutdown and a
// non-nil error on unexpected serve failures.
func (h HTTPServer) serveOne(srv *http.Server, rl resolvedListener) (err error) {
	logger := h.logger.WithValues("addr", rl.addr, "network", networkOf(rl))
	var ln net.Listener
	if ln, err = newListener(rl); err != nil {
		logger.Error(err, "listen failed")
		return
	}
	defer func() {
		if closeErr := ln.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			logger.Error(closeErr, "close listener failed")
		}
	}()
	if rl.tlsCertPath != "" && rl.tlsKeyPath != "" {
		var tlsCfg *tls.Config
		tlsCfg, err = tlsConfig(logger, rl)
		if err != nil {
			logger.Error(err, "load TLS config failed")
			return
		}
		ln = tls.NewListener(ln, tlsCfg)
	}
	logger.Info("serving")
	if err = srv.Serve(ln); errors.Is(err, http.ErrServerClosed) {
		err = nil
	} else if err != nil {
		logger.Error(err, "serve failed")
	}
	logger.Info("stopped")
	return
}

// newListener creates a net.Listener for the given resolved listener.
// Network defaults to "tcp" when rl.network is empty.
func newListener(rl resolvedListener) (net.Listener, error) {
	return net.Listen(networkOf(rl), rl.addr)
}

func networkOf(rl resolvedListener) string {
	if rl.network != "" {
		return rl.network
	}
	return "tcp"
}

// tlsConfig returns a tls.Config whose server certificate is served by a
// pkg/tls CertReloader: the cert/key are loaded from rl up front (failing fast
// on a bad path) and transparently reloaded on the next handshake after the
// files are rotated on disk, so cert-manager renewals take effect without a restart.
// When rl.clientCAPath is set it additionally enables mutual TLS, requiring and
// verifying client certificates against that CA bundle, which is likewise
// reloaded on rotation (via GetConfigForClient) without a restart. An explicit
// rl.tlsMinVersion overrides the default minimum TLS version.
func tlsConfig(logger logr.Logger, rl resolvedListener) (cfg *tls.Config, err error) {
	certReloader, err := tlsreload.NewCertReloader(logger, rl.tlsCertPath, rl.tlsKeyPath)
	if err != nil {
		return
	}
	cfg = &tls.Config{GetCertificate: certReloader.GetCertificate}
	// Advertise ALPN protocols so HTTP/2 (and thus gRPC) can be negotiated over
	// TLS. The Go http2 server only injects "h2" automatically when it builds the
	// TLS listener itself; here we wrap the listener with our own tls.Config, so we
	// must set NextProtos explicitly. gRPC clients (>= grpc-go 1.67) enforce that
	// "h2" was negotiated and otherwise fail with "missing selected ALPN property".
	// "http/1.1" is kept as a fallback so plain REST/Gin traffic still works.
	cfg.NextProtos = []string{"h2", "http/1.1"}
	if rl.clientCAPath != "" {
		var caReloader *tlsreload.CAReloader
		caReloader, err = tlsreload.NewCAReloader(logger, rl.clientCAPath)
		if err != nil {
			return
		}
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cfg.MinVersion = tls.VersionTLS13 // mTLS branch only, does not affect the default listener
		// Seed ClientCAs so the base config is self-consistent, but the value
		// served per handshake comes from GetConfigForClient below, which picks
		// up rotations of the CA bundle without a restart.
		cfg.ClientCAs = caReloader.CurrentPool()
		cfg.GetConfigForClient = caReloader.ConfigForClient(cfg)
	}
	if rl.tlsMinVersion != 0 {
		cfg.MinVersion = rl.tlsMinVersion
	}
	return
}

var (
	httpFailure = metrics.Register(func(factory promauto.Factory) *prometheus.CounterVec {
		return factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_failure",
			Help: "The total number of http failures",
		}, []string{"status_code", "component"})
	})
)

func ginLogger(rootLogger logr.Logger) gin.HandlerFunc {
	logger := rootLogger.WithName("gin")
	return func(ctx *gin.Context) {
		path := ctx.Request.URL.Path
		rawQuery := ctx.Request.URL.RawQuery
		begin := time.Now()
		ctx.Next()
		end := time.Now()
		latency := end.Sub(begin)
		clientIP := ctx.ClientIP()
		method := ctx.Request.Method
		statusCode := ctx.Writer.Status()
		if rawQuery != "" {
			path = path + "?" + rawQuery
		}
		var logErr error
		for _, e := range ctx.Errors {
			logErr = errors.Join(logErr, e)
		}
		if logErr != nil {
			httpFailure.WithLabelValues(strconv.Itoa(statusCode), version.Name).Inc()
			if rawBody, exist := ctx.Get(gin.BodyBytesKey); exist {
				if body, ok := rawBody.([]byte); ok {
					logger.Error(logErr, "handle request failed",
						"status", statusCode, "latency", latency,
						"clientIp", clientIP, "method", method, "path", path, "requestBody", body)
					return
				}
			}
			logger.Error(logErr, "handle request failed",
				"status", statusCode, "latency", latency,
				"clientIp", clientIP, "method", method, "path", path)
		} else {
			if utils.IsNoLog(ctx) {
				return
			}
			if withBody := logger.V(5); withBody.Enabled() {
				if rawBody, exist := ctx.Get(gin.BodyBytesKey); exist {
					if body, ok := rawBody.([]byte); ok {
						withBody.Info("request",
							"status", statusCode, "latency", latency,
							"clientIp", clientIP, "method", method, "path", path, "requestBody", body)
						return
					}
				}
			}
			logger.V(4).Info("request",
				"status", statusCode, "latency", latency,
				"clientIp", clientIP, "method", method, "path", path)
		}
	}
}
