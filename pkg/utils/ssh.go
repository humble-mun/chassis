package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"golang.org/x/crypto/ssh"
)

// GenerateSSHKeyPair generates a new RSA SSH key pair
func GenerateSSHKeyPair() (public, private string, err error) {
	var privateKey *rsa.PrivateKey
	if privateKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
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
