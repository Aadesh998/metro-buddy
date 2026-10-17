package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"math/big"
)

type Box struct {
	aead    cipher.AEAD
	version int16
}

func NewBox(key []byte) (*Box, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead, version: 1}, nil
}

func (b *Box) KeyVersion() int16 { return b.version }

func (b *Box) Seal(plain []byte) []byte {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return b.aead.Seal(nonce, nonce, plain, nil)
}

func (b *Box) SealString(s string) []byte { return b.Seal([]byte(s)) }

func (b *Box) Open(sealed []byte) ([]byte, error) {
	n := b.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("ciphertext too short")
	}
	return b.aead.Open(nil, sealed[:n], sealed[n:], nil)
}

func (b *Box) OpenString(sealed []byte) (string, error) {
	p, err := b.Open(sealed)
	return string(p), err
}

func HMAC(key []byte, parts ...string) []byte {
	m := hmac.New(sha256.New, key)
	for _, p := range parts {
		m.Write([]byte(p))
		m.Write([]byte{0})
	}
	return m.Sum(nil)
}

func SHA256(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}

func Token(nbytes int) string {
	b := make([]byte, nbytes)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func Digits(n int) string {
	out := make([]byte, n)
	for i := range out {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			panic(err)
		}
		out[i] = byte('0' + d.Int64())
	}
	return string(out)
}

func Equal(a, b []byte) bool { return hmac.Equal(a, b) }
