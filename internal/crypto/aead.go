package crypto

import (
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
)

// ErrOpen is returned when a ciphertext fails authentication. It deliberately
// carries no detail about why.
var ErrOpen = errors.New("crypto: message authentication failed")

// Seal encrypts and authenticates plaintext with XChaCha20-Poly1305 under a
// fresh random nonce. The result is nonce ‖ ciphertext ‖ tag.
func Seal(key, plaintext, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := RandomBytes(chacha20poly1305.NonceSizeX)
	out := make([]byte, 0, len(nonce)+len(plaintext)+aead.Overhead())
	out = append(out, nonce...)
	return aead.Seal(out, nonce, plaintext, aad), nil
}

// Open reverses Seal. The caller should Wipe the returned plaintext when done.
func Open(key, blob, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	if len(blob) < chacha20poly1305.NonceSizeX+aead.Overhead() {
		return nil, ErrOpen
	}
	nonce, ct := blob[:chacha20poly1305.NonceSizeX], blob[chacha20poly1305.NonceSizeX:]
	pt, err := aead.Open(nil, nonce, ct, aad)
	if err != nil {
		return nil, ErrOpen
	}
	return pt, nil
}
