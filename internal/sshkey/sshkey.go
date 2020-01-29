package sshkey

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"time"

	"github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"golang.org/x/crypto/ssh"
)

const (
	DefaultBlockSize = 3072
)

type Key struct {
	Private string
	Public  string
}

type Factory struct {
	context   context.Context
	queue     chan *Key
	blockSize int
}

func NewFactory(context context.Context, queueSize int, blockSize int) *Factory {
	return &Factory{
		context:   context,
		queue:     make(chan *Key, queueSize),
		blockSize: blockSize,
	}
}

func (factory *Factory) Run() {
	for {
		select {
		case <-factory.context.Done():
			return
		default:
		}

		key, err := Generate(factory.blockSize)
		if err != nil {
			log.Errorf(
				err,
				"sshkey.Factory: unable to generate key with block size; %d",
				factory.blockSize,
			)

			time.Sleep(time.Second)

			continue
		}

		select {
		case <-factory.context.Done():
			return
		case factory.queue <- key:
		}
	}
}

func (factory *Factory) Get() chan *Key {
	return factory.queue
}

func Generate(blockSize int) (*Key, error) {
	private, err := generatePrivateKey(blockSize)
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
