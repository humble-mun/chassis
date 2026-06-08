package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
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
	"github.com/humble-mun/chassis/pkg/service"
	"github.com/humble-mun/chassis/pkg/utils"
)

// HTTPServer is an HTTP server implementation using the HTTPServer framework
type HTTPServer struct {
	logger    logr.Logger
	engine    *gin.Engine
	grpc      *grpc.Server
	listeners []listenerConfig
}

// RegisterRoute registers custom routes with the Gin engine
func (h HTTPServer) RegisterRoute(api func(engine *gin.Engine)) {
	api(h.engine)
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

	ginHandler := h.engine.Handler()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.grpc != nil && r.ProtoMajor == 2 && strings.HasPrefix(
			r.Header.Get("Content-Type"), "application/grpc") {
			h.grpc.ServeHTTP(w, r)
			return
		}
		ginHandler.ServeHTTP(w, r)
	})

	servers := make([]*http.Server, len(listeners))
	for i, lc := range listeners {
		if lc.addr == "" {
			err = fmt.Errorf("listener %d (%s): %w", i, lc.network, ErrAddrMissing)
			return
		}
		srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
		p := new(http.Protocols)
		if lc.tlsCertPath != "" && lc.tlsKeyPath != "" {
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

	// Start a listener goroutine per listenerConfig.
	for i, lc := range listeners {
		srv := servers[i]
		group.Go(func() error {
			return h.serveOne(srv, lc)
		})
	}

	// Shutdown all servers when ctx is cancelled.
	group.Go(func() error {
		<-groupCtx.Done()
		sc, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for i, srv := range servers {
			lc := listeners[i]
			logger := h.logger.WithValues("addr", lc.addr, "network", networkOf(lc))
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

// serveOne opens a net.Listener for lc, wraps it in TLS when configured, and
// runs srv until it is shut down. It returns nil on clean shutdown and a
// non-nil error on unexpected serve failures.
func (h HTTPServer) serveOne(srv *http.Server, lc listenerConfig) (err error) {
	logger := h.logger.WithValues("addr", lc.addr, "network", networkOf(lc))
	var ln net.Listener
	if ln, err = newListener(lc); err != nil {
		logger.Error(err, "listen failed")
		return
	}
	defer func() {
		if closeErr := ln.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			logger.Error(closeErr, "close listener failed")
		}
	}()
	if lc.tlsCertPath != "" && lc.tlsKeyPath != "" {
		var tlsCfg *tls.Config
		tlsCfg, err = tlsConfig(lc)
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

// newListener creates a net.Listener for the given listenerConfig.
// Network defaults to "tcp" when lc.network is empty.
func newListener(lc listenerConfig) (net.Listener, error) {
	return net.Listen(networkOf(lc), lc.addr)
}

func networkOf(lc listenerConfig) string {
	if lc.network != "" {
		return lc.network
	}
	return "tcp"
}

// tlsConfig loads the server certificate and key from lc and returns a
// tls.Config. When lc.clientCAPath is set it additionally enables mutual TLS,
// requiring and verifying client certificates against that CA bundle. An
// explicit lc.tlsMinVersion overrides the default minimum TLS version.
func tlsConfig(lc listenerConfig) (cfg *tls.Config, err error) {
	var cert tls.Certificate
	cert, err = tls.LoadX509KeyPair(lc.tlsCertPath, lc.tlsKeyPath)
	if err != nil {
		return
	}
	cfg = &tls.Config{Certificates: []tls.Certificate{cert}}
	if lc.clientCAPath != "" {
		var caPEM []byte
		if caPEM, err = os.ReadFile(lc.clientCAPath); err != nil {
			err = fmt.Errorf("read client CA %q: %w", lc.clientCAPath, err)
			return
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			err = fmt.Errorf("parse client CA from %q", lc.clientCAPath)
			return
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
		cfg.MinVersion = tls.VersionTLS13 // mTLS branch only, does not affect the default listener
	}
	if lc.tlsMinVersion != 0 {
		cfg.MinVersion = lc.tlsMinVersion
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
			httpFailure.WithLabelValues(strconv.Itoa(statusCode), service.ConfigName).Inc()
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
