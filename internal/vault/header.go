package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/awnumar/memguard"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
)

const (
	headerFile = "vault.header"
	dbFile     = "vault.db"

	formatName    = "pangolin-vault"
	formatVersion = 1
	kdfAlgorithm  = "argon2id"
	wrapAlgorithm = "xchacha20-poly1305"

	// A header is a few hundred bytes. Refuse to parse anything much larger.
	maxHeaderSize = 64 * 1024
)

// header is the on-disk key slot. It is the only unencrypted file of a vault
// and contains nothing secret.
type header struct {
	Format  string     `json:"format"`
	Version int        `json:"version"`
	KDF     headerKDF  `json:"kdf"`
	Wrap    headerWrap `json:"wrap"`
}

type headerKDF struct {
	Algorithm string `json:"algorithm"`
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memory_kib"`
	Threads   uint8  `json:"threads"`
	Salt      []byte `json:"salt"`
}

type headerWrap struct {
	Algorithm  string `json:"algorithm"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

func (h *header) params() crypto.KDFParams {
	return crypto.KDFParams{Time: h.KDF.Time, MemoryKiB: h.KDF.MemoryKiB, Threads: h.KDF.Threads}
}

// aad is a canonical encoding of every header field that is not itself
// ciphertext, so none of them can be changed without the unwrap failing.
func (h *header) aad() []byte {
	s := fmt.Sprintf("%s\x00%d\x00%s\x00%d\x00%d\x00%d\x00%s\x00",
		h.Format, h.Version, h.KDF.Algorithm, h.KDF.Time, h.KDF.MemoryKiB, h.KDF.Threads, h.Wrap.Algorithm)
	return append([]byte(s), h.KDF.Salt...)
}

// newHeader wraps master under a key derived from password.
func newHeader(password, master []byte, p crypto.KDFParams) (*header, error) {
	h := &header{
		Format:  formatName,
		Version: formatVersion,
		KDF: headerKDF{
			Algorithm: kdfAlgorithm,
			Time:      p.Time,
			MemoryKiB: p.MemoryKiB,
			Threads:   p.Threads,
			Salt:      crypto.RandomBytes(crypto.SaltSize),
		},
		Wrap: headerWrap{Algorithm: wrapAlgorithm},
	}
	kek, err := crypto.DeriveKEK(password, h.KDF.Salt, p)
	if err != nil {
		return nil, err
	}
	defer kek.Destroy()

	blob, err := crypto.Seal(kek.Bytes(), master, h.aad())
	if err != nil {
		return nil, err
	}
	h.Wrap.Nonce = blob[:chacha20poly1305.NonceSizeX]
	h.Wrap.Ciphertext = blob[chacha20poly1305.NonceSizeX:]
	return h, nil
}

// unwrap recovers the master key. Every failure that could be caused by a
// wrong password or a modified header is reported as ErrUnlock.
func (h *header) unwrap(password []byte) (*memguard.LockedBuffer, error) {
	if h.Format != formatName || h.KDF.Algorithm != kdfAlgorithm || h.Wrap.Algorithm != wrapAlgorithm {
		return nil, ErrUnlock
	}
	if h.Version != formatVersion {
		return nil, fmt.Errorf("vault: unsupported header version %d", h.Version)
	}
	kek, err := crypto.DeriveKEK(password, h.KDF.Salt, h.params())
	if err != nil {
		return nil, ErrUnlock
	}
	defer kek.Destroy()

	blob := append(append([]byte{}, h.Wrap.Nonce...), h.Wrap.Ciphertext...)
	master, err := crypto.Open(kek.Bytes(), blob, h.aad())
	if err != nil || len(master) != crypto.KeySize {
		return nil, ErrUnlock
	}
	return memguard.NewBufferFromBytes(master), nil
}

func readHeader(dir string) (*header, error) {
	path := filepath.Join(dir, headerFile)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoVault
	}
	if err != nil {
		return nil, err
	}
	if info.Size() > maxHeaderSize {
		return nil, ErrUnlock
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var h header
	if err := json.Unmarshal(raw, &h); err != nil {
		return nil, ErrUnlock
	}
	return &h, nil
}

// writeHeader replaces the header atomically: a crash leaves either the old
// file or the new one, never a partial write.
func writeHeader(dir string, h *header) error {
	raw, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, headerFile+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, headerFile))
}
