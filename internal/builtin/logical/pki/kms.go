package pki

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/mldsa"
	"crypto/rsa"
	"fmt"
	"io"

	"github.com/openbao/go-kms-wrapping/v2/kms"
	"golang.org/x/crypto/ed25519"
)

type kmsSigner struct {
	ctx context.Context
	key kms.Key
	pub crypto.PublicKey
}

func (s *kmsSigner) Public() crypto.PublicKey {
	return s.pub
}

func (s *kmsSigner) Sign(
	rand io.Reader,
	digest []byte,
	opts crypto.SignerOpts,
) ([]byte, error) {
	if opts == nil {
		opts = crypto.Hash(0)
	}

	hash := opts.HashFunc()

	signature, err := s.key.Sign(s.ctx, &kms.SignOptions{
		Data:       digest,
		Prehashed:  hash != crypto.Hash(0),
		SignerOpts: opts,
	})
	return signature, err

}

func (s *kmsSigner) SignMessage(
	rand io.Reader,
	message []byte,
	opts crypto.SignerOpts,
) ([]byte, error) {
	if opts == nil {
		opts = crypto.Hash(0)
	}

	signature, err := s.key.Sign(s.ctx, &kms.SignOptions{
		Data:       message,
		Prehashed:  false,
		SignerOpts: opts,
	})
	return signature, err
}

func NewKMSSigner(ctx context.Context, key kms.Key) (crypto.Signer, error) {
	pub, err := key.ExportPublic(ctx)
	if err != nil {
		return nil, err
	}

	switch pub.(type) {
	case *rsa.PublicKey, *ecdsa.PublicKey, ed25519.PublicKey, *mldsa.PublicKey:
	default:
		return nil, fmt.Errorf("unsupported KMS public key type %T", pub)
	}

	return &kmsSigner{
		ctx: ctx,
		key: key,
		pub: pub,
	}, nil
}
