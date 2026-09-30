// Package crypto wraps the small set of primitives Pangolin relies on:
// Argon2id, HKDF-SHA-256 and XChaCha20-Poly1305. Nothing else in the
// application touches a primitive directly.
package crypto

import (
	"errors"
	"fmt"

	"github.com/awnumar/memguard"
	"golang.org/x/crypto/argon2"
)

const (
	// KeySize is the size in bytes of every symmetric key in Pangolin.
	KeySize = 32
	// SaltSize is the size in bytes of the Argon2id salt.
	SaltSize = 16
)

// Bounds for KDF parameters read from a vault header. They stop a manipulated
// header from requesting an absurd allocation or a trivially weak derivation.
const (
	minMemoryKiB = 8 * 1024
	maxMemoryKiB = 4 * 1024 * 1024
	minTime      = 1
	maxTime      = 64
	minThreads   = 1
	maxThreads   = 64
)

// KDFParams are the Argon2id cost parameters.
type KDFParams struct {
	Time      uint32
	MemoryKiB uint32
	Threads   uint8
}

// DefaultKDFParams returns the parameters used for new vaults: four times the
// memory and one more pass than the second recommended set of RFC 9106
// (64 MiB, 3 passes), which a desktop derives in well under a second.
func DefaultKDFParams() KDFParams {
	return KDFParams{Time: 4, MemoryKiB: 256 * 1024, Threads: 4}
}

// Validate reports whether p is within the accepted bounds.
func (p KDFParams) Validate() error {
	switch {
	case p.Time < minTime || p.Time > maxTime:
		return fmt.Errorf("crypto: argon2id time %d out of range", p.Time)
	case p.MemoryKiB < minMemoryKiB || p.MemoryKiB > maxMemoryKiB:
		return fmt.Errorf("crypto: argon2id memory %d KiB out of range", p.MemoryKiB)
	case p.Threads < minThreads || p.Threads > maxThreads:
		return fmt.Errorf("crypto: argon2id threads %d out of range", p.Threads)
	}
	return nil
}

// DeriveKEK stretches password into a key-encryption key. The caller owns the
// returned buffer and must Destroy it.
func DeriveKEK(password, salt []byte, p KDFParams) (*memguard.LockedBuffer, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if len(salt) != SaltSize {
		return nil, errors.New("crypto: bad salt size")
	}
	if len(password) == 0 {
		return nil, errors.New("crypto: empty password")
	}
	key := argon2.IDKey(password, salt, p.Time, p.MemoryKiB, p.Threads, KeySize)
	// NewBufferFromBytes wipes key after copying it into locked memory.
	return memguard.NewBufferFromBytes(key), nil
}
