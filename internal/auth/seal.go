package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

// sealer encrypts and authenticates small values with the session key.
// purpose is bound to each value, so one sealed for the sign-in cookie can't
// be passed off as a stored token, or the other way round.
type sealer struct{ aead cipher.AEAD }

func newSealer(key []byte) (*sealer, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &sealer{aead}, nil
}

func (s *sealer) seal(purpose string, plain []byte) []byte {
	nonce := make([]byte, s.aead.NonceSize())
	rand.Read(nonce)
	return s.aead.Seal(nonce, nonce, plain, []byte(purpose))
}

func (s *sealer) open(purpose string, sealed []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("sealed value too short")
	}
	return s.aead.Open(nil, sealed[:n], sealed[n:], []byte(purpose))
}
