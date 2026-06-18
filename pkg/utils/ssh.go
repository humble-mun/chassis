package utils

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
)

// GenerateSSHKeyPair generates a new RSA SSH key pair with the given key size in bits.
func GenerateSSHKeyPair(keySize int) (public, private string, err error) {
	var privateKey *rsa.PrivateKey
	if privateKey, err = rsa.GenerateKey(rand.Reader, keySize); err != nil {
		return
	}
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	private = string(privateKeyPEM)
	var publicKey ssh.PublicKey
	if publicKey, err = ssh.NewPublicKey(&privateKey.PublicKey); err != nil {
		return
	}
	public = string(ssh.MarshalAuthorizedKey(publicKey))
	return
}

// GenerateEd25519KeyPair generates a new Ed25519 SSH key pair.
func GenerateEd25519KeyPair() (public, private string, err error) {
	var publicKeyRaw ed25519.PublicKey
	var privateKeyRaw ed25519.PrivateKey
	if publicKeyRaw, privateKeyRaw, err = ed25519.GenerateKey(rand.Reader); err != nil {
		return
	}
	var privateKeyPEM *pem.Block
	if privateKeyPEM, err = ssh.MarshalPrivateKey(privateKeyRaw, ""); err != nil {
		return
	}
	private = string(pem.EncodeToMemory(privateKeyPEM))
	var publicKey ssh.PublicKey
	if publicKey, err = ssh.NewPublicKey(publicKeyRaw); err != nil {
		return
	}
	public = string(ssh.MarshalAuthorizedKey(publicKey))
	return
}
