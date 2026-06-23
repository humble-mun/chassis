package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	cryptotls "crypto/tls"
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

// genCertPEM returns a self-signed ECDSA P-256 leaf certificate and its key,
// both PEM-encoded, with the given common name.
func genCertPEM(t *testing.T, commonName string) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

// writePair writes a certificate/key pair to the given paths.
func writePair(t *testing.T, certPath, keyPath string, certPEM, keyPEM []byte) {
	t.Helper()
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
}

// leafCN returns the common name of the leaf certificate currently served by r.
func leafCN(t *testing.T, r *CertReloader) string {
	t.Helper()
	cert, err := r.GetCertificate(&cryptotls.ClientHelloInfo{})
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	return leaf.Subject.CommonName
}

func TestCertReloaderInitialLoad(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	certPEM, keyPEM := genCertPEM(t, "gen-A")
	writePair(t, certPath, keyPath, certPEM, keyPEM)

	r, err := NewCertReloader(logr.Discard(), certPath, keyPath)
	if err != nil {
		t.Fatalf("NewCertReloader: %v", err)
	}
	if got := leafCN(t, r); got != "gen-A" {
		t.Fatalf("leaf CN = %q, want gen-A", got)
	}
}

func TestCertReloaderMissingFailsFast(t *testing.T) {
	dir := t.TempDir()
	_, err := NewCertReloader(logr.Discard(), filepath.Join(dir, "absent.crt"), filepath.Join(dir, "absent.key"))
	if err == nil {
		t.Fatal("expected error for missing cert/key, got nil")
	}
}

func TestCertReloaderHotReload(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	certA, keyA := genCertPEM(t, "gen-A")
	writePair(t, certPath, keyPath, certA, keyA)

	r, err := NewCertReloader(logr.Discard(), certPath, keyPath)
	if err != nil {
		t.Fatalf("NewCertReloader: %v", err)
	}
	if got := leafCN(t, r); got != "gen-A" {
		t.Fatalf("before rotation: leaf CN = %q, want gen-A", got)
	}

	certB, keyB := genCertPEM(t, "gen-B")
	writePair(t, certPath, keyPath, certB, keyB)
	// Bump mtime so rotation is detectable even if the new files happen to
	// share inode/size with the old ones (e.g. on platforms without inodes).
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(certPath, future, future); err != nil {
		t.Fatalf("chtimes cert: %v", err)
	}
	if err := os.Chtimes(keyPath, future, future); err != nil {
		t.Fatalf("chtimes key: %v", err)
	}

	if got := leafCN(t, r); got != "gen-B" {
		t.Fatalf("after rotation: leaf CN = %q, want gen-B", got)
	}
}

func TestCertReloaderFallbackOnBadReload(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	certA, keyA := genCertPEM(t, "gen-A")
	writePair(t, certPath, keyPath, certA, keyA)

	r, err := NewCertReloader(logr.Discard(), certPath, keyPath)
	if err != nil {
		t.Fatalf("NewCertReloader: %v", err)
	}

	// Corrupt the cert file and bump mtime: reload must fail and the previous
	// certificate must keep being served.
	if err := os.WriteFile(certPath, []byte("not a pem"), 0o600); err != nil {
		t.Fatalf("corrupt cert: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(certPath, future, future); err != nil {
		t.Fatalf("chtimes cert: %v", err)
	}

	if got := leafCN(t, r); got != "gen-A" {
		t.Fatalf("after bad reload: leaf CN = %q, want gen-A (previous)", got)
	}
}

func TestCertReloaderConcurrentRotation(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")
	certA, keyA := genCertPEM(t, "gen-A")
	writePair(t, certPath, keyPath, certA, keyA)

	r, err := NewCertReloader(logr.Discard(), certPath, keyPath)
	if err != nil {
		t.Fatalf("NewCertReloader: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				if _, err := r.GetCertificate(&cryptotls.ClientHelloInfo{}); err != nil {
					t.Errorf("GetCertificate: %v", err)
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 20; j++ {
			certPEM, keyPEM := genCertPEM(t, "rot")
			writePair(t, certPath, keyPath, certPEM, keyPEM)
			future := time.Now().Add(time.Duration(j+1) * time.Second)
			_ = os.Chtimes(certPath, future, future)
			_ = os.Chtimes(keyPath, future, future)
		}
	}()
	wg.Wait()
}
