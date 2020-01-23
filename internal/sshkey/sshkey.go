package sshkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/reconquest/karma-go"
	"golang.org/x/crypto/ssh"
)

const (
	BlockSize = 4096
)

type Key struct {
	Private string
	Public  string
}

func Generate() (*Key, error) {
	private, err := generatePrivateKey(BlockSize)
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to generate private part",
		)
	}

	public, err := generatePublicKey(&private.PublicKey)
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to generate public part",
		)
	}

	privatePEM := marshalPrivateKeyToPEM(private)

	return &Key{Private: string(privatePEM), Public: string(public)}, nil
}

func generatePrivateKey(bitSize int) (*rsa.PrivateKey, error) {
	private, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		return nil, err
	}

	err = private.Validate()
	if err != nil {
		return nil, err
	}

	return private, nil
}

func marshalPrivateKeyToPEM(private *rsa.PrivateKey) []byte {
	der := x509.MarshalPKCS1PrivateKey(private)

	block := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   der,
	}

	return pem.EncodeToMemory(&block)
}

func generatePublicKey(private *rsa.PublicKey) ([]byte, error) {
	public, err := ssh.NewPublicKey(private)
	if err != nil {
		return nil, err
	}

	return ssh.MarshalAuthorizedKey(public), nil
}
