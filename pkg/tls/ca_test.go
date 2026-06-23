package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

// genCA returns a self-signed CA certificate, PEM-encoded, plus the parsed
// certificate for verification.
func genCA(t *testing.T, cn string) (caPEM []byte, ca *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}
	ca, err = x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA: %v", err)
	}
	caPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return caPEM, ca
}

// trusts reports whether the pool currently served by r trusts ca.
func trusts(t *testing.T, r *CAReloader, ca *x509.Certificate) bool {
	t.Helper()
	pool := r.CurrentPool()
	_, err := ca.Verify(x509.VerifyOptions{Roots: pool})
	return err == nil
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestCAReloaderInitialLoad(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	caPEM, ca := genCA(t, "ca-A")
	writeFile(t, caPath, caPEM)

	r, err := NewCAReloader(logr.Discard(), caPath)
	if err != nil {
		t.Fatalf("NewCAReloader: %v", err)
	}
	if !trusts(t, r, ca) {
		t.Fatal("pool does not trust ca-A")
	}
}

func TestCAReloaderRejectsBadBundle(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	writeFile(t, caPath, []byte("not a pem"))

	if _, err := NewCAReloader(logr.Discard(), caPath); err == nil {
		t.Fatal("expected error for invalid CA bundle, got nil")
	}
}

func TestCAReloaderHotReload(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	caAPEM, caA := genCA(t, "ca-A")
	writeFile(t, caPath, caAPEM)

	r, err := NewCAReloader(logr.Discard(), caPath)
	if err != nil {
		t.Fatalf("NewCAReloader: %v", err)
	}
	if !trusts(t, r, caA) {
		t.Fatal("before rotation: pool does not trust ca-A")
	}

	caBPEM, caB := genCA(t, "ca-B")
	writeFile(t, caPath, caBPEM)
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(caPath, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if !trusts(t, r, caB) {
		t.Fatal("after rotation: pool does not trust ca-B")
	}
	if trusts(t, r, caA) {
		t.Fatal("after rotation: pool still trusts ca-A")
	}
}

func TestCAReloaderFallbackOnBadReload(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	caAPEM, caA := genCA(t, "ca-A")
	writeFile(t, caPath, caAPEM)

	r, err := NewCAReloader(logr.Discard(), caPath)
	if err != nil {
		t.Fatalf("NewCAReloader: %v", err)
	}

	writeFile(t, caPath, []byte("not a pem"))
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(caPath, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if !trusts(t, r, caA) {
		t.Fatal("after bad reload: pool no longer trusts ca-A (previous)")
	}
}

func TestCAReloaderConcurrentRotation(t *testing.T) {
	dir := t.TempDir()
	caPath := filepath.Join(dir, "ca.crt")
	caAPEM, _ := genCA(t, "ca-A")
	writeFile(t, caPath, caAPEM)

	r, err := NewCAReloader(logr.Discard(), caPath)
	if err != nil {
		t.Fatalf("NewCAReloader: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if r.CurrentPool() == nil {
					t.Error("CurrentPool returned nil")
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			caPEM, _ := genCA(t, "rot")
			writeFile(t, caPath, caPEM)
			future := time.Now().Add(time.Duration(j+1) * time.Second)
			_ = os.Chtimes(caPath, future, future)
		}
	}()
	wg.Wait()
}
