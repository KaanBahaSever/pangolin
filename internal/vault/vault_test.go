package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
)

var (
	testParams = crypto.KDFParams{Time: 1, MemoryKiB: 8 * 1024, Threads: 1}
	testPass   = []byte("correct horse battery")
)

func newVault(t *testing.T) (*Vault, string) {
	t.Helper()
	dir := t.TempDir()
	v, err := Create(dir, testPass, testParams)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { v.Close() })
	return v, dir
}

func mustAdd(t *testing.T, v *Vault, e Entry) string {
	t.Helper()
	id, err := v.Add(e)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCreateOpenRoundTrip(t *testing.T) {
	v, dir := newVault(t)
	id := mustAdd(t, v, Entry{
		Title: "GitHub", Username: "octocat", URL: "https://github.com",
		Password: []byte("hunter2-plaintext"), Notes: []byte("recovery codes"),
	})
	if err := v.Close(); err != nil {
		t.Fatal(err)
	}

	v2, err := Open(dir, testPass)
	if err != nil {
		t.Fatal(err)
	}
	defer v2.Close()

	list, err := v2.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != id || list[0].Title != "GitHub" || list[0].Username != "octocat" || !list[0].HasNotes {
		t.Fatalf("unexpected list: %+v", list)
	}
	pw, err := v2.Secret(id, FieldPassword)
	if err != nil || string(pw) != "hunter2-plaintext" {
		t.Fatalf("password = %q, %v", pw, err)
	}
	notes, err := v2.Secret(id, FieldNotes)
	if err != nil || string(notes) != "recovery codes" {
		t.Fatalf("notes = %q, %v", notes, err)
	}
}

func TestWrongPassword(t *testing.T) {
	v, dir := newVault(t)
	v.Close()
	if _, err := Open(dir, []byte("incorrect horse battery")); !errors.Is(err, ErrUnlock) {
		t.Fatalf("err = %v, want ErrUnlock", err)
	}
}

func TestOpenWithoutVault(t *testing.T) {
	if _, err := Open(t.TempDir(), testPass); !errors.Is(err, ErrNoVault) {
		t.Fatalf("err = %v, want ErrNoVault", err)
	}
}

func TestCreateRefusesWeakPasswordAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	if _, err := Create(dir, []byte("short"), testParams); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("err = %v, want ErrWeakPassword", err)
	}
	if Exists(dir) {
		t.Fatal("a rejected Create left a vault behind")
	}
	v, err := Create(dir, testPass, testParams)
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	if _, err := Create(dir, testPass, testParams); !errors.Is(err, ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
}

func TestCreateMovesOrphanDatabaseAside(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, dbFile), []byte("leftover"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := Create(dir, testPass, testParams)
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
	orphans, _ := filepath.Glob(filepath.Join(dir, dbFile+".orphan-*"))
	if len(orphans) != 1 {
		t.Fatalf("orphans = %v", orphans)
	}
	if raw, _ := os.ReadFile(orphans[0]); string(raw) != "leftover" {
		t.Fatal("orphan content changed")
	}
}

// Every change to the header, whether to a parameter or to ciphertext, must
// make unlocking fail.
func TestHeaderTampering(t *testing.T) {
	v, dir := newVault(t)
	v.Close()
	path := filepath.Join(dir, headerFile)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	mutations := map[string]func(h *header){
		"time":       func(h *header) { h.KDF.Time++ },
		"memory":     func(h *header) { h.KDF.MemoryKiB += 1024 },
		"threads":    func(h *header) { h.KDF.Threads++ },
		"salt":       func(h *header) { h.KDF.Salt[0] ^= 1 },
		"nonce":      func(h *header) { h.Wrap.Nonce[0] ^= 1 },
		"ciphertext": func(h *header) { h.Wrap.Ciphertext[0] ^= 1 },
		"format":     func(h *header) { h.Format = "other" },
		"kdf":        func(h *header) { h.KDF.Algorithm = "pbkdf2" },
		"huge":       func(h *header) { h.KDF.MemoryKiB = 1 << 31 },
	}
	for name, mutate := range mutations {
		var h header
		if err := json.Unmarshal(original, &h); err != nil {
			t.Fatal(err)
		}
		mutate(&h)
		raw, _ := json.Marshal(&h)
		os.WriteFile(path, raw, 0o600)
		if _, err := Open(dir, testPass); !errors.Is(err, ErrUnlock) {
			t.Errorf("%s: err = %v, want ErrUnlock", name, err)
		}
	}

	os.WriteFile(path, []byte("not json"), 0o600)
	if _, err := Open(dir, testPass); !errors.Is(err, ErrUnlock) {
		t.Errorf("garbage header: err = %v, want ErrUnlock", err)
	}

	os.WriteFile(path, original, 0o600)
	v2, err := Open(dir, testPass)
	if err != nil {
		t.Fatalf("restored header: %v", err)
	}
	v2.Close()
}

func TestDatabaseTampering(t *testing.T) {
	v, dir := newVault(t)
	mustAdd(t, v, Entry{Title: "a", Password: []byte("p")})
	v.Close()

	path := filepath.Join(dir, dbFile)
	raw, _ := os.ReadFile(path)
	raw[len(raw)/2] ^= 0xff
	raw[100] ^= 0xff
	os.WriteFile(path, raw, 0o600)

	v2, err := Open(dir, testPass)
	if err == nil {
		_, err = v2.List("")
		v2.Close()
	}
	if err == nil {
		t.Fatal("a modified database was read without error")
	}
}

// The raw files must not contain anything the user typed.
func TestNoPlaintextOnDisk(t *testing.T) {
	v, dir := newVault(t)
	needles := []string{"UniqueTitle-7731", "unique.user-7731", "unique-url-7731.example", "UniquePassw0rd-7731", "UniqueNotes-7731"}
	mustAdd(t, v, Entry{
		Title: needles[0], Username: needles[1], URL: needles[2],
		Password: []byte(needles[3]), Notes: []byte(needles[4]),
	})
	v.Close()

	files, _ := os.ReadDir(dir)
	if len(files) != 2 {
		t.Fatalf("expected exactly the header and the database, got %d files", len(files))
	}
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, n := range append(needles, string(testPass), "SQLite format 3", "CREATE TABLE") {
			if bytes.Contains(raw, []byte(n)) {
				t.Errorf("%s contains plaintext %q", f.Name(), n)
			}
		}
	}
}

func TestUpdateDeleteAndSearch(t *testing.T) {
	v, _ := newVault(t)
	a := mustAdd(t, v, Entry{Title: "Mail", Username: "me@example.com", Password: []byte("one")})
	b := mustAdd(t, v, Entry{Title: "github", Username: "octocat", URL: "github.com", Password: []byte("two")})
	mustAdd(t, v, Entry{Title: "Çiçek Bank", Password: []byte("three")})

	list, _ := v.List("")
	if len(list) != 3 {
		t.Fatalf("len = %d", len(list))
	}
	if list[0].Title != "github" || list[1].Title != "Mail" {
		t.Fatalf("not sorted case-insensitively: %q, %q", list[0].Title, list[1].Title)
	}
	for query, want := range map[string]string{"GITHUB": b, "example.com": a, " octo ": b} {
		got, _ := v.List(query)
		if len(got) != 1 || got[0].ID != want {
			t.Errorf("List(%q) = %+v", query, got)
		}
	}
	if got, _ := v.List("çiçek"); len(got) != 1 {
		t.Errorf("non-ASCII case-insensitive search found %d entries", len(got))
	}
	mustAdd(t, v, Entry{Title: "İstanbul Kart", Username: "IŞIK"})
	for _, query := range []string{"istanbul", "İSTANBUL", "ışık", "işik"} {
		if got, _ := v.List(query); len(got) != 1 {
			t.Errorf("Turkish search %q found %d entries", query, len(got))
		}
	}
	if got, _ := v.List("nothing"); len(got) != 0 {
		t.Errorf("unexpected match: %+v", got)
	}
	if list[1].HasNotes {
		t.Error("entry without notes reports HasNotes")
	}

	if err := v.Update(Entry{ID: a, Title: "Mail 2", Username: "new", Password: []byte("changed"), Notes: []byte("n")}); err != nil {
		t.Fatal(err)
	}
	pw, _ := v.Secret(a, FieldPassword)
	if string(pw) != "changed" {
		t.Fatalf("password after update = %q", pw)
	}
	if err := v.Update(Entry{ID: "missing", Title: "x"}); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := v.Update(Entry{ID: a, Title: "  "}); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("err = %v", err)
	}
	if _, err := v.Add(Entry{Title: ""}); !errors.Is(err, ErrInvalidEntry) {
		t.Fatalf("err = %v", err)
	}

	if err := v.Delete(a); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Secret(a, FieldPassword); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := v.Delete(a); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("err = %v", err)
	}
	if _, err := v.Secret(b, Field("title")); err == nil {
		t.Fatal("unknown field was accepted")
	}
}

// A sealed value copied to another entry or another column must not decrypt.
func TestCiphertextIsBoundToEntryAndField(t *testing.T) {
	v, _ := newVault(t)
	a := mustAdd(t, v, Entry{Title: "a", Password: []byte("pw-a"), Notes: []byte("notes-a")})
	b := mustAdd(t, v, Entry{Title: "b", Password: []byte("pw-b")})

	if _, err := v.db.Exec("UPDATE entries SET password = (SELECT password FROM entries WHERE id = ?) WHERE id = ?", a, b); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Secret(b, FieldPassword); !errors.Is(err, crypto.ErrOpen) {
		t.Fatalf("swapped between entries: err = %v", err)
	}
	if _, err := v.db.Exec("UPDATE entries SET password = notes WHERE id = ?", a); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Secret(a, FieldPassword); !errors.Is(err, crypto.ErrOpen) {
		t.Fatalf("swapped between fields: err = %v", err)
	}
}

func TestChangePassword(t *testing.T) {
	v, dir := newVault(t)
	id := mustAdd(t, v, Entry{Title: "a", Password: []byte("kept")})
	next := []byte("a whole new password")

	if err := v.ChangePassword([]byte("not the current one"), next); !errors.Is(err, ErrUnlock) {
		t.Fatalf("err = %v, want ErrUnlock", err)
	}
	if err := v.ChangePassword(testPass, []byte("short")); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("err = %v, want ErrWeakPassword", err)
	}
	if err := v.ChangePassword(testPass, next); err != nil {
		t.Fatal(err)
	}
	v.Close()

	if _, err := Open(dir, testPass); !errors.Is(err, ErrUnlock) {
		t.Fatalf("old password: err = %v, want ErrUnlock", err)
	}
	v2, err := Open(dir, next)
	if err != nil {
		t.Fatal(err)
	}
	defer v2.Close()
	if pw, _ := v2.Secret(id, FieldPassword); string(pw) != "kept" {
		t.Fatalf("password after change = %q", pw)
	}
	if tmp, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(tmp) != 0 {
		t.Fatalf("temporary files left behind: %v", tmp)
	}
}

func TestLockedVaultRefusesEverything(t *testing.T) {
	v, _ := newVault(t)
	id := mustAdd(t, v, Entry{Title: "a"})
	v.Close()
	if err := v.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := v.List(""); !errors.Is(err, ErrLocked) {
		t.Errorf("List: %v", err)
	}
	if _, err := v.Secret(id, FieldPassword); !errors.Is(err, ErrLocked) {
		t.Errorf("Secret: %v", err)
	}
	if _, err := v.Add(Entry{Title: "b"}); !errors.Is(err, ErrLocked) {
		t.Errorf("Add: %v", err)
	}
	if err := v.Update(Entry{ID: id, Title: "b"}); !errors.Is(err, ErrLocked) {
		t.Errorf("Update: %v", err)
	}
	if err := v.Delete(id); !errors.Is(err, ErrLocked) {
		t.Errorf("Delete: %v", err)
	}
	if err := v.ChangePassword(testPass, testPass); !errors.Is(err, ErrLocked) {
		t.Errorf("ChangePassword: %v", err)
	}
}

func TestOpenWithMissingDatabase(t *testing.T) {
	v, dir := newVault(t)
	v.Close()
	os.Remove(filepath.Join(dir, dbFile))
	if _, err := Open(dir, testPass); err == nil {
		t.Fatal("opened a vault whose database is missing")
	}
	if _, err := os.Stat(filepath.Join(dir, dbFile)); err == nil {
		t.Fatal("Open created a new empty database")
	}
}
