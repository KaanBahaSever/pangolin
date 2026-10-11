package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// Random and mutated ciphertexts must never crash Open, and must never be
// accepted unless they are exactly what Seal produced.
func FuzzOpen(f *testing.F) {
	key := bytes.Repeat([]byte{7}, KeySize)
	aad := []byte("pangolin/v1/field\x00id\x00password")
	// A fixed message sealed under key: the fuzzer runs this setup again in
	// every worker process, and Seal picks a random nonce each time.
	valid, _ := hex.DecodeString("7acaf1f19e4a765d96278844a9bcf4ac2ffee7ccedba09d0b29cc49f48c0fe3da8af04e08188af000b449b3400871c")
	if pt, err := Open(key, valid, aad); err != nil || string(pt) != "hunter2" {
		f.Fatal("the reference message does not open")
	}
	f.Add(valid, aad)
	f.Add([]byte{}, []byte{})
	f.Add(valid[:NonceSize+Overhead], aad)

	f.Fuzz(func(t *testing.T, blob, ad []byte) {
		pt, err := Open(key, blob, ad)
		if err != nil {
			return
		}
		if !bytes.Equal(blob, valid) || !bytes.Equal(ad, aad) || string(pt) != "hunter2" {
			t.Fatalf("forged message accepted: blob %x aad %q -> %q", blob, ad, pt)
		}
	})
}
