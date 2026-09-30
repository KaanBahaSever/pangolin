package vault

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
)

// Field names a secret column of an entry.
type Field string

// The secret fields of an entry.
const (
	FieldPassword Field = "password"
	FieldNotes    Field = "notes"
)

// sealedEmpty is the size of a sealed empty value: nonce plus tag.
const sealedEmpty = 24 + 16

// Entry is a full entry as supplied by the caller for Add and Update.
// Password and Notes are plaintext; the vault seals them before storing.
type Entry struct {
	ID       string
	Title    string
	Username string
	URL      string
	Password []byte
	Notes    []byte
}

// Summary is the non-secret part of an entry, used for listing and search.
type Summary struct {
	ID        string
	Title     string
	Username  string
	URL       string
	HasNotes  bool
	UpdatedAt time.Time
}

// turkishI maps the dotted and dotless I of Turkish onto plain "i", so that
// "istanbul" finds "İstanbul" and "isik" finds "IŞIK" whatever the locale.
var turkishI = strings.NewReplacer("İ", "i", "ı", "i")

// fold prepares a string for case-insensitive comparison.
func fold(s string) string {
	return strings.ToLower(turkishI.Replace(s))
}

func fieldAAD(id string, f Field) []byte {
	return []byte("pangolin/v1/field\x00" + id + "\x00" + string(f))
}

func (v *Vault) seal(id string, f Field, plaintext []byte) ([]byte, error) {
	return crypto.Seal(v.fieldKey.Bytes(), plaintext, fieldAAD(id, f))
}

// List returns the summaries of all entries whose title, username or URL
// contains query, ignoring case, ordered by title. An empty query matches all.
func (v *Vault) List(query string) ([]Summary, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, ErrLocked
	}
	rows, err := v.db.Query(`SELECT id, title, username, url, length(notes), updated_at
	                         FROM entries ORDER BY title COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Filtering happens here, not in SQL: SQLite's LIKE only folds ASCII case.
	query = fold(strings.TrimSpace(query))
	var out []Summary
	for rows.Next() {
		var s Summary
		var notesLen int
		var updated int64
		if err := rows.Scan(&s.ID, &s.Title, &s.Username, &s.URL, &notesLen, &updated); err != nil {
			return nil, err
		}
		s.HasNotes = notesLen > sealedEmpty
		s.UpdatedAt = time.Unix(updated, 0)
		if query == "" ||
			strings.Contains(fold(s.Title), query) ||
			strings.Contains(fold(s.Username), query) ||
			strings.Contains(fold(s.URL), query) {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

// Secret decrypts one secret field of an entry. The caller must crypto.Wipe
// the result as soon as it has been used.
func (v *Vault) Secret(id string, f Field) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return nil, ErrLocked
	}
	// The column name comes from a fixed set, never from input.
	var column string
	switch f {
	case FieldPassword:
		column = "password"
	case FieldNotes:
		column = "notes"
	default:
		return nil, errors.New("vault: unknown field")
	}
	var blob []byte
	err := v.db.QueryRow("SELECT "+column+" FROM entries WHERE id = ?", id).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEntryNotFound
	}
	if err != nil {
		return nil, err
	}
	return crypto.Open(v.fieldKey.Bytes(), blob, fieldAAD(id, f))
}

// Add stores a new entry and returns its ID. e.ID is ignored.
func (v *Vault) Add(e Entry) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return "", ErrLocked
	}
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		return "", ErrInvalidEntry
	}
	id := hex.EncodeToString(crypto.RandomBytes(16))
	password, err := v.seal(id, FieldPassword, e.Password)
	if err != nil {
		return "", err
	}
	notes, err := v.seal(id, FieldNotes, e.Notes)
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	_, err = v.db.Exec(`INSERT INTO entries (id, title, username, url, password, notes, created_at, updated_at)
	                    VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, e.Title, strings.TrimSpace(e.Username), strings.TrimSpace(e.URL), password, notes, now, now)
	if err != nil {
		return "", err
	}
	return id, nil
}

// Update replaces every field of the entry with ID e.ID.
func (v *Vault) Update(e Entry) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return ErrLocked
	}
	e.Title = strings.TrimSpace(e.Title)
	if e.Title == "" {
		return ErrInvalidEntry
	}
	password, err := v.seal(e.ID, FieldPassword, e.Password)
	if err != nil {
		return err
	}
	notes, err := v.seal(e.ID, FieldNotes, e.Notes)
	if err != nil {
		return err
	}
	res, err := v.db.Exec(`UPDATE entries SET title = ?, username = ?, url = ?, password = ?, notes = ?, updated_at = ?
	                       WHERE id = ?`,
		e.Title, strings.TrimSpace(e.Username), strings.TrimSpace(e.URL), password, notes, time.Now().Unix(), e.ID)
	if err != nil {
		return err
	}
	return requireOneRow(res)
}

// Delete removes an entry.
func (v *Vault) Delete(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.db == nil {
		return ErrLocked
	}
	res, err := v.db.Exec("DELETE FROM entries WHERE id = ?", id)
	if err != nil {
		return err
	}
	return requireOneRow(res)
}

func requireOneRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrEntryNotFound
	}
	return nil
}
