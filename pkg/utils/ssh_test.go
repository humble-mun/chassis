package utils

import (
	"crypto/ed25519"
	"crypto/rsa"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestGenerateSSHKeyPair(t *testing.T) {
	public, private, err := GenerateSSHKeyPair(2048)
	if err != nil {
		t.Fatalf("GenerateSSHKeyPair(2048) returned error: %v", err)
	}

	if !strings.HasPrefix(private, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Errorf("private key is not in PKCS#1 PEM format:\n%s", private)
	}

	signer, err := ssh.ParsePrivateKey([]byte(private))
	if err != nil {
		t.Fatalf("private key is not parseable: %v", err)
	}
	if _, ok := signer.PublicKey().(ssh.CryptoPublicKey).CryptoPublicKey().(*rsa.PublicKey); !ok {
		t.Errorf("expected an RSA public key, got %T", signer.PublicKey())
	}

	parsedPub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(public))
	if err != nil {
		t.Fatalf("public key is not parseable: %v", err)
	}
	if parsedPub.Type() != ssh.KeyAlgoRSA {
		t.Errorf("public key type = %q, want %q", parsedPub.Type(), ssh.KeyAlgoRSA)
	}
}

func TestGenerateSSHKeyPairInvalidKeySize(t *testing.T) {
	if _, _, err := GenerateSSHKeyPair(128); err == nil {
		t.Error("expected error for too-small RSA key size, got nil")
	}
}

func TestGenerateEd25519KeyPair(t *testing.T) {
	public, private, err := GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair() returned error: %v", err)
	}

	if !strings.HasPrefix(private, "-----BEGIN OPENSSH PRIVATE KEY-----") {
		t.Errorf("private key is not in OpenSSH PEM format:\n%s", private)
	}

	signer, err := ssh.ParsePrivateKey([]byte(private))
	if err != nil {
		t.Fatalf("private key is not parseable: %v", err)
	}
	if _, ok := signer.PublicKey().(ssh.CryptoPublicKey).CryptoPublicKey().(ed25519.PublicKey); !ok {
		t.Errorf("expected an Ed25519 public key, got %T", signer.PublicKey())
	}

	parsedPub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(public))
	if err != nil {
		t.Fatalf("public key is not parseable: %v", err)
	}
	if parsedPub.Type() != ssh.KeyAlgoED25519 {
		t.Errorf("public key type = %q, want %q", parsedPub.Type(), ssh.KeyAlgoED25519)
	}
}
