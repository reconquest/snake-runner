package sshkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"

	"github.com/reconquest/karma-go"
	"golang.org/x/crypto/ssh"
)

func GeneratePair(path string, size int) error {
	private, err := generatePrivateKey(size)
	if err != nil {
		return karma.Format(
			err,
			"unable to generate private part",
		)
	}

	public, err := generatePublicKey(&private.PublicKey)
	if err != nil {
		return karma.Format(
			err,
			"unable to generate public part",
		)
	}

	privatePEM := marshalPrivateKeyToPEM(private)

	err = ioutil.WriteFile(path, privatePEM, 0600)
	if err != nil {
		return karma.Format(
			err,
			"unable to write private part to file",
		)
	}

	err = ioutil.WriteFile(path+".pub", public, 0600)
	if err != nil {
		return karma.Format(
			err,
			"unable to write public part to file: %s", path+".pub",
		)
	}

	return nil
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
