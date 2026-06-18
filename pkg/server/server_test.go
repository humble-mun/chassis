package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
)

func TestRegisterFlags(t *testing.T) {
	pfs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	RegisterFlags(pfs)

	for _, name := range []string{flagHTTPBindAddress, flagTLSCertPath, flagTLSKeyPath} {
		if pfs.Lookup(name) == nil {
			t.Fatalf("expected flag %q to be registered", name)
		}
	}
}

func TestListenerOptions(t *testing.T) {
	o := &options{}
	WithTCPListener(
		WithAddr(func() string { return "127.0.0.1:9000" }),
		WithTLSCert(func() string { return "cert" }, func() string { return "key" }),
		WithMTLS(func() string { return "ca" }),
		WithTLSMinVersion(tls.VersionTLS12),
	)(o)

	if len(o.listeners) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(o.listeners))
	}

	rl := resolve(o.listeners[0])
	if rl.addr != "127.0.0.1:9000" {
		t.Errorf("addr = %q, want 127.0.0.1:9000", rl.addr)
	}
	if rl.network != "tcp" {
		t.Errorf("network = %q, want tcp", rl.network)
	}
	if rl.tlsCertPath != "cert" || rl.tlsKeyPath != "key" {
		t.Errorf("tls paths = %q/%q, want cert/key", rl.tlsCertPath, rl.tlsKeyPath)
	}
	if rl.clientCAPath != "ca" {
		t.Errorf("clientCAPath = %q, want ca", rl.clientCAPath)
	}
	if rl.tlsMinVersion != tls.VersionTLS12 {
		t.Errorf("tlsMinVersion = %d, want %d", rl.tlsMinVersion, tls.VersionTLS12)
	}
}

func TestWithUnixListener(t *testing.T) {
	o := &options{}
	WithUnixListener(WithAddr(func() string { return "/tmp/sock" }))(o)

	if len(o.listeners) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(o.listeners))
	}
	rl := resolve(o.listeners[0])
	if rl.network != "unix" {
		t.Errorf("network = %q, want unix", rl.network)
	}
	if networkOf(rl) != "unix" {
		t.Errorf("networkOf = %q, want unix", networkOf(rl))
	}
}

func TestResolveNilProvidersYieldEmpty(t *testing.T) {
	rl := resolve(listenerConfig{})
	if rl.addr != "" || rl.tlsCertPath != "" || rl.tlsKeyPath != "" || rl.clientCAPath != "" {
		t.Errorf("expected empty resolved listener, got %+v", rl)
	}
	if networkOf(rl) != "tcp" {
		t.Errorf("networkOf default = %q, want tcp", networkOf(rl))
	}
}

func TestWithGRPCServerAndReadHeaderTimeout(t *testing.T) {
	o := &options{}
	grpcSrv := grpc.NewServer()
	WithGRPCServer(grpcSrv)(o)
	WithReadHeaderTimeout(3 * time.Second)(o)

	if o.grpc != grpcSrv {
		t.Error("expected grpc server to be set")
	}
	if o.readHeaderTimeout != 3*time.Second {
		t.Errorf("readHeaderTimeout = %v, want 3s", o.readHeaderTimeout)
	}
}

func TestWithDefaultCORSConfig(t *testing.T) {
	o := &options{}
	WithDefaultCORSConfig()(o)
	if o.corsConfig == nil {
		t.Fatal("expected cors config to be set")
	}
	if o.corsConfig.AllowAllOrigins {
		t.Error("default cors config should not allow all origins")
	}

	o = &options{}
	WithDefaultCORSConfig(WithCORSAllowAllOrigins())(o)
	if !o.corsConfig.AllowAllOrigins || !o.corsConfig.AllowCredentials {
		t.Error("expected WithCORSAllowAllOrigins to relax the policy")
	}
}

func TestNewHTTPServerDefaultCORSDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// The bare default policy permits no cross-origin request; constructing the
	// server must succeed and simply omit the CORS middleware rather than panic.
	srv := NewHTTPServer(logr.Discard(), WithDefaultCORSConfig())
	srv.RegisterRoute(func(engine *gin.Engine) {
		engine.GET("/ping", func(ctx *gin.Context) { ctx.String(http.StatusOK, "pong") })
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.Header.Set("Origin", "https://cross-origin.test")
	srv.engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS header under default policy, got %q", got)
	}
}

func TestNewHTTPServerAllowAllOriginsInstallsCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := NewHTTPServer(logr.Discard(), WithDefaultCORSConfig(WithCORSAllowAllOrigins()))
	srv.RegisterRoute(func(engine *gin.Engine) {
		engine.GET("/ping", func(ctx *gin.Context) { ctx.String(http.StatusOK, "pong") })
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.Header.Set("Origin", "https://cross-origin.test")
	srv.engine.ServeHTTP(response, request)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Error("expected CORS middleware to set Access-Control-Allow-Origin")
	}
}

func TestNewHTTPServerServesRegisteredRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	srv := NewHTTPServer(logr.Discard(), WithDefaultCORSConfig())
	srv.RegisterRoute(func(engine *gin.Engine) {
		engine.GET("/ping", func(ctx *gin.Context) {
			ctx.String(http.StatusOK, "pong")
		})
	})

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	srv.engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Body.String() != "pong" {
		t.Fatalf("body = %q, want pong", response.Body.String())
	}
}

func TestStartReturnsErrAddrMissing(t *testing.T) {
	srv := NewHTTPServer(logr.Discard(), WithTCPListener())

	err := srv.Start(context.Background())
	if !errors.Is(err, ErrAddrMissing) {
		t.Fatalf("Start error = %v, want ErrAddrMissing", err)
	}
}

func TestStartGracefulShutdown(t *testing.T) {
	srv := NewHTTPServer(logr.Discard(), WithTCPListener(
		WithAddr(func() string { return "127.0.0.1:0" }),
	))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	// Give the listener time to bind before triggering shutdown.
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned error on graceful shutdown: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

func TestTLSConfig(t *testing.T) {
	certPath, keyPath := writeSelfSignedCert(t)

	t.Run("server cert only", func(t *testing.T) {
		cfg, err := tlsConfig(resolvedListener{tlsCertPath: certPath, tlsKeyPath: keyPath})
		if err != nil {
			t.Fatalf("tlsConfig error: %v", err)
		}
		if len(cfg.Certificates) != 1 {
			t.Fatalf("expected 1 certificate, got %d", len(cfg.Certificates))
		}
		wantProtos := []string{"h2", "http/1.1"}
		if len(cfg.NextProtos) != len(wantProtos) {
			t.Fatalf("NextProtos = %v, want %v", cfg.NextProtos, wantProtos)
		}
		if cfg.ClientAuth != tls.NoClientCert {
			t.Errorf("ClientAuth = %v, want NoClientCert", cfg.ClientAuth)
		}
	})

	t.Run("mTLS enables client verification", func(t *testing.T) {
		cfg, err := tlsConfig(resolvedListener{
			tlsCertPath:  certPath,
			tlsKeyPath:   keyPath,
			clientCAPath: certPath,
		})
		if err != nil {
			t.Fatalf("tlsConfig error: %v", err)
		}
		if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
			t.Errorf("ClientAuth = %v, want RequireAndVerifyClientCert", cfg.ClientAuth)
		}
		if cfg.ClientCAs == nil {
			t.Error("expected ClientCAs to be populated")
		}
		if cfg.MinVersion != tls.VersionTLS13 {
			t.Errorf("MinVersion = %d, want TLS1.3", cfg.MinVersion)
		}
	})

	t.Run("explicit min version override", func(t *testing.T) {
		cfg, err := tlsConfig(resolvedListener{
			tlsCertPath:   certPath,
			tlsKeyPath:    keyPath,
			tlsMinVersion: tls.VersionTLS12,
		})
		if err != nil {
			t.Fatalf("tlsConfig error: %v", err)
		}
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Errorf("MinVersion = %d, want TLS1.2", cfg.MinVersion)
		}
	})

	t.Run("missing cert file", func(t *testing.T) {
		_, err := tlsConfig(resolvedListener{
			tlsCertPath: filepath.Join(t.TempDir(), "missing.crt"),
			tlsKeyPath:  keyPath,
		})
		if err == nil {
			t.Fatal("expected error for missing certificate file")
		}
	})

	t.Run("missing client CA file", func(t *testing.T) {
		_, err := tlsConfig(resolvedListener{
			tlsCertPath:  certPath,
			tlsKeyPath:   keyPath,
			clientCAPath: filepath.Join(t.TempDir(), "missing-ca.crt"),
		})
		if err == nil {
			t.Fatal("expected error for missing client CA file")
		}
	})
}

func writeSelfSignedCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "chassis-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	dir := t.TempDir()
	certPath = filepath.Join(dir, "tls.crt")
	keyPath = filepath.Join(dir, "tls.key")

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err = os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err = os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return
}
