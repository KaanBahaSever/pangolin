// Package vault implements the encrypted store: the key slot, the SQLCipher
// database and per-field encryption of secrets. See architecture.md §5–§7.
package vault

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/awnumar/memguard"
	"golang.org/x/text/unicode/norm"

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
	// ErrMissingDatabase means the header is there but vault.db is not.
	ErrMissingDatabase = errors.New("vault: the database file is missing")
	// ErrEntryNotFound means no entry has the requested ID.
	ErrEntryNotFound = errors.New("vault: entry not found")
	// ErrInvalidEntry means an entry failed validation.
	ErrInvalidEntry = errors.New("vault: an entry needs a title")
)

// Vault is an unlocked vault. It is safe for concurrent use. Close it to lock.
type Vault struct {
	mu       sync.Mutex
	headerMu sync.Mutex // serialises header rewrites; held without mu
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

	pw := nfc(password)
	defer crypto.Wipe(pw)
	master := crypto.NewMasterKey()
	h, err := newHeader(pw, master.Bytes(), params)
	if err != nil {
		master.Destroy()
		return nil, err
	}
	v, err := open(dir, master, true)
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
		return nil, fmt.Errorf("%w: %v", ErrMissingDatabase, err)
	}
	master, err := unwrap(h, password)
	if err != nil {
		return nil, err
	}
	v, err := open(dir, master, false)
	if err != nil {
		// The password was right but the database is not usable. This is
		// reported like a wrong password, so failures do not tell an
		// observer which of the two was at fault; the cause stays in the
		// error chain.
		return nil, fmt.Errorf("%w (%v)", ErrUnlock, err)
	}
	return v, nil
}

// nfc returns a copy of password in Unicode normalisation form C, so that a
// password with "ş" or "é" gives the same bytes however the keyboard or the
// operating system composed those letters. The caller wipes the copy.
func nfc(password []byte) []byte {
	return norm.NFC.Append(make([]byte, 0, len(password)), password...)
}

// unwrap recovers the master key. It tries the password in NFC first and then
// exactly as typed, because vaults made before 1.3 used the raw bytes.
func unwrap(h *header, password []byte) (*memguard.LockedBuffer, error) {
	pw := nfc(password)
	defer crypto.Wipe(pw)
	master, err := h.unwrap(pw)
	if errors.Is(err, ErrUnlock) && !bytes.Equal(pw, password) {
		return h.unwrap(password)
	}
	return master, err
}

// open takes ownership of master.
func open(dir string, master *memguard.LockedBuffer, create bool) (*Vault, error) {
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
	db, err := openDB(filepath.Join(dir, dbFile), dbKey.Bytes(), create)
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
//
// The two key derivations take a while, so they run without holding the
// vault's lock: locking the vault meanwhile does not have to wait for them.
func (v *Vault) ChangePassword(current, next []byte) error {
	if utf8.RuneCount(next) < MinPasswordLength {
		return ErrWeakPassword
	}
	v.mu.Lock()
	if v.db == nil {
		v.mu.Unlock()
		return ErrLocked
	}
	master := memguard.NewBuffer(crypto.KeySize)
	master.Copy(v.master.Bytes())
	v.mu.Unlock()
	defer master.Destroy()

	v.headerMu.Lock()
	defer v.headerMu.Unlock()
	old, err := readHeader(v.dir)
	if err != nil {
		return err
	}
	check, err := unwrap(old, current)
	if err != nil {
		return err
	}
	check.Destroy()

	pw := nfc(next)
	defer crypto.Wipe(pw)
	h, err := newHeader(pw, master.Bytes(), old.params())
	if err != nil {
		return err
	}
	return writeHeader(v.dir, h)
}
