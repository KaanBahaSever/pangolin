package crypto

import (
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"

	"github.com/awnumar/memguard"
)

// HKDF info strings. Changing one changes the derived key, so they are part
// of the vault format.
const (
	InfoDatabase = "pangolin/v1/database"
	InfoFields   = "pangolin/v1/fields"
)

// NewMasterKey returns a fresh random master key in locked memory.
func NewMasterKey() *memguard.LockedBuffer {
	return memguard.NewBufferRandom(KeySize)
}

// DeriveSubkey derives an independent key from the master key for the given
// purpose. The caller owns the returned buffer and must Destroy it.
func DeriveSubkey(master []byte, info string) (*memguard.LockedBuffer, error) {
	key, err := hkdf.Key(sha256.New, master, nil, info, KeySize)
	if err != nil {
		return nil, err
	}
	return memguard.NewBufferFromBytes(key), nil
}

// RandomBytes returns n bytes from the operating system CSPRNG.
func RandomBytes(n int) []byte {
	b := make([]byte, n)
	// crypto/rand.Read never returns an error; it aborts the program instead.
	rand.Read(b)
	return b
}

// Wipe overwrites b with zeros.
func Wipe(b []byte) {
	memguard.WipeBytes(b)
}
