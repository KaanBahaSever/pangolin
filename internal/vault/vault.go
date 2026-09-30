// Package vault implements the encrypted store: the key slot, the SQLCipher
// database and per-field encryption of secrets. See architecture.md §5–§7.
package vault

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/awnumar/memguard"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
)

// MinPasswordLength is the minimum master password length, in characters.
const MinPasswordLength = 12

var (
	// ErrNoVault means there is no vault in the directory.
	ErrNoVault = errors.New("vault: no vault found")
	// ErrExists means a vault is already present and will not be overwritten.
	ErrExists = errors.New("vault: a vault already exists")
	// ErrUnlock covers a wrong master password and a damaged header alike.
	ErrUnlock = errors.New("vault: wrong master password or damaged vault")
	// ErrLocked is returned by every operation on a closed vault.
	ErrLocked = errors.New("vault: locked")
	// ErrWeakPassword means the master password is shorter than MinPasswordLength.
	ErrWeakPassword = fmt.Errorf("vault: master password must be at least %d characters", MinPasswordLength)
	// ErrEntryNotFound means no entry has the requested ID.
	ErrEntryNotFound = errors.New("vault: entry not found")
	// ErrInvalidEntry means an entry failed validation.
	ErrInvalidEntry = errors.New("vault: an entry needs a title")
)

// Vault is an unlocked vault. It is safe for concurrent use. Close it to lock.
type Vault struct {
	mu       sync.Mutex
	dir      string
	db       *sql.DB
	master   *memguard.LockedBuffer
	fieldKey *memguard.LockedBuffer
}

// Exists reports whether dir contains a vault.
func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, headerFile))
	return err == nil
}

// Create makes a new, empty vault in dir protected by password and returns it
// unlocked. It never overwrites an existing vault.
func Create(dir string, password []byte, params crypto.KDFParams) (*Vault, error) {
	if utf8.RuneCount(password) < MinPasswordLength {
		return nil, ErrWeakPassword
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if Exists(dir) {
		return nil, ErrExists
	}
	// A database without a header is left over from an interrupted Create or
	// a lost header. Its key is unknown here, so move it aside, not delete it.
	dbPath := filepath.Join(dir, dbFile)
	if _, err := os.Stat(dbPath); err == nil {
		orphan := fmt.Sprintf("%s.orphan-%d", dbPath, time.Now().Unix())
		if err := os.Rename(dbPath, orphan); err != nil {
			return nil, err
		}
	}

	master := crypto.NewMasterKey()
	h, err := newHeader(password, master.Bytes(), params)
	if err != nil {
		master.Destroy()
		return nil, err
	}
	v, err := open(dir, master)
	if err != nil {
		return nil, err
	}
	// The header is written last: a vault "exists" only once it is complete.
	if err := writeHeader(dir, h); err != nil {
		v.Close()
		return nil, err
	}
	return v, nil
}

// Open unlocks the vault in dir.
func Open(dir string, password []byte) (*Vault, error) {
	h, err := readHeader(dir)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, dbFile)); err != nil {
		return nil, fmt.Errorf("vault: database file is missing: %w", err)
	}
	master, err := h.unwrap(password)
	if err != nil {
		return nil, err
	}
	return open(dir, master)
}

// open takes ownership of master.
func open(dir string, master *memguard.LockedBuffer) (*Vault, error) {
	dbKey, err := crypto.DeriveSubkey(master.Bytes(), crypto.InfoDatabase)
	if err != nil {
		master.Destroy()
		return nil, err
	}
	defer dbKey.Destroy()

	fieldKey, err := crypto.DeriveSubkey(master.Bytes(), crypto.InfoFields)
	if err != nil {
		master.Destroy()
		return nil, err
	}
	db, err := openDB(filepath.Join(dir, dbFile), dbKey.Bytes())
	if err != nil {
		master.Destroy()
		fieldKey.Destroy()
		return nil, err
	}
	return &Vault{dir: dir, db: db, master: master, fieldKey: fieldKey}, nil
}

// Close locks the vault: it closes the database and destroys every key.
// It is safe to call more than once.
func (v *Vault) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil
	}
	err := v.db.Close()
	v.db = nil
	v.master.Destroy()
	v.fieldKey.Destroy()
	return err
}

// ChangePassword re-wraps the master key under a new password. The current
// password is required again, so an unattended unlocked session is not enough
// to take over the vault.
func (v *Vault) ChangePassword(current, next []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return ErrLocked
	}
	if utf8.RuneCount(next) < MinPasswordLength {
		return ErrWeakPassword
	}
	old, err := readHeader(v.dir)
	if err != nil {
		return err
	}
	check, err := old.unwrap(current)
	if err != nil {
		return err
	}
	check.Destroy()

	h, err := newHeader(next, v.master.Bytes(), old.params())
	if err != nil {
		return err
	}
	return writeHeader(v.dir, h)
}
