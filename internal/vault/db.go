package vault

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"

	_ "github.com/mutecomm/go-sqlcipher/v4" // registers the "sqlite3" driver
)

const schemaVersion = 1

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
func openDB(path string, key []byte) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s?_pragma_key=x'%s'&_pragma_cipher_page_size=4096", path, hex.EncodeToString(key))
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	// One connection: the per-connection pragmas below must apply to every
	// statement, and a password keeper has no use for a pool.
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA cipher_memory_security = ON",
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

	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	// SQLite creates the file world-readable on Unix. It is encrypted, but
	// there is no reason to share it.
	os.Chmod(path, 0o600)
	return db, nil
}

func migrate(db *sql.DB) error {
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
