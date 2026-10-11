package vault

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	_ "github.com/mutecomm/go-sqlcipher/v4" // registers the "sqlite3" driver
)

const schemaVersion = 1

var errEmptyDatabase = errors.New("vault: the database is empty or damaged")

const schemaV1 = `
CREATE TABLE entries (
  id         TEXT PRIMARY KEY,
  title      TEXT NOT NULL,
  username   TEXT NOT NULL DEFAULT '',
  url        TEXT NOT NULL DEFAULT '',
  password   BLOB NOT NULL,
  notes      BLOB NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);`

// openDB opens the SQLCipher database at path with a raw 256-bit key and
// brings its schema up to date.
//
// The driver only accepts the key as part of a string, so the database key
// passes through memory that cannot be wiped. See architecture.md §7.4.
//
// create is true only when a new vault is being made. Opening an existing
// vault never creates the schema: a database without one is empty or
// damaged (a truncated copy, a sync conflict) and must not be mistaken for a
// vault with no entries.
func openDB(path string, key []byte, create bool) (*sql.DB, error) {
	if err := memorySecurityOff(); err != nil {
		return nil, err
	}
	// The driver splits its connection string at the first "?". A "?" in the
	// path would cut the key off and create an unencrypted database.
	if strings.ContainsRune(path, '?') {
		return nil, errors.New(`vault: the vault folder's path must not contain "?"`)
	}
	dsn := fmt.Sprintf("%s?_pragma_key=x'%s'&_pragma_cipher_page_size=4096", path, hex.EncodeToString(key))
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	// One connection: the per-connection pragmas below must apply to every
	// statement, and a password keeper has no use for a pool.
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA secure_delete = ON",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA journal_mode = DELETE",
		"PRAGMA foreign_keys = ON",
	} {
		// journal_mode returns a row, so Query rather than Exec.
		rows, err := db.Query(pragma)
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("vault: database cannot be opened: %w", err)
		}
		rows.Close()
	}

	if err := migrate(db, create); err != nil {
		db.Close()
		return nil, err
	}
	// SQLite creates the file world-readable on Unix. It is encrypted, but
	// there is no reason to share it.
	os.Chmod(path, 0o600)
	return db, nil
}

var (
	memoryOnce sync.Once
	memoryErr  error
)

// memorySecurityOff turns SQLCipher's memory locking off, once per process
// and before any vault is opened, because the setting is global and applies
// to allocations made while a database is keyed.
//
// That feature locks every SQLite allocation into RAM. The operating system
// allows a process to lock very little (about 44 pages on Windows by
// default), and SQLCipher used up almost all of it. memguard then failed to
// lock a key buffer, which is fatal: changing the master password could
// crash the app. The keys matter more than SQLite's page cache, which holds
// titles, usernames, URLs and sealed secrets only. SQLCipher still wipes its
// own key material when a database closes. See architecture.md §7.1.
func memorySecurityOff() error {
	memoryOnce.Do(func() {
		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			memoryErr = err
			return
		}
		defer db.Close()
		if _, err := db.Exec("PRAGMA cipher_memory_security = OFF"); err != nil {
			memoryErr = fmt.Errorf("vault: cannot configure SQLCipher: %w", err)
		}
	})
	return memoryErr
}

func migrate(db *sql.DB, create bool) error {
	var version int
	// Reading user_version is also the first real read of the file, so a
	// wrong key or a damaged database surfaces here.
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("vault: database cannot be opened: %w", err)
	}
	if version > schemaVersion {
		return fmt.Errorf("vault: database schema %d is newer than this version of Pangolin supports", version)
	}
	if version == schemaVersion {
		return nil
	}
	if !create {
		return errEmptyDatabase
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(schemaV1); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}
