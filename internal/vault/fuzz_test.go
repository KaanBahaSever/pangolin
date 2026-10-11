package vault

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fuzzVault is a small vault built once; each fuzz input works on a copy.
func fuzzVault(f *testing.F) (hdr, db []byte, secret string) {
	f.Helper()
	dir := f.TempDir()
	v, err := Create(dir, testPass, testParams)
	if err != nil {
		f.Fatal(err)
	}
	if _, err := v.Add(Entry{Title: "fuzz", Username: "user", Password: []byte("the secret"), Notes: []byte("notes")}); err != nil {
		f.Fatal(err)
	}
	v.Close()
	hdr, _ = os.ReadFile(filepath.Join(dir, headerFile))
	db, _ = os.ReadFile(filepath.Join(dir, dbFile))
	return hdr, db, "the secret"
}

// checkOpen opens a vault made of the given files and checks that whatever
// opens gives either an error or exactly the original data.
func checkOpen(t *testing.T, hdr, db []byte, secret string) {
	// Not t.TempDir: on Windows a virus scanner may briefly hold a file the
	// fuzzer just wrote, and a failed cleanup would count as a failed input.
	dir, err := os.MkdirTemp("", "pangolin-fuzz-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, headerFile), hdr, 0o600)
	os.WriteFile(filepath.Join(dir, dbFile), db, 0o600)
	v, err := Open(dir, testPass)
	if err != nil {
		return
	}
	defer v.Close()
	list, err := v.List("")
	if err != nil {
		return
	}
	for _, s := range list {
		pw, err := v.Secret(s.ID, FieldPassword)
		if err == nil && string(pw) != secret {
			t.Fatalf("a modified vault decrypted to %q", pw)
		}
	}
}

// A modified header must never crash, and must never unlock with a different
// key. Inputs asking for an expensive derivation are skipped, so the fuzzer
// spends its time on parsing rather than on Argon2id.
func FuzzHeader(f *testing.F) {
	original, db, secret := fuzzVault(f)
	f.Add(original)
	f.Add([]byte("{}"))
	f.Add([]byte(`{"format":"pangolin-vault","version":1}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		var h header
		if json.Unmarshal(raw, &h) == nil && (h.KDF.MemoryKiB > 32*1024 || h.KDF.Time > 2 || h.KDF.Threads > 2) {
			t.Skip("expensive parameters")
		}
		checkOpen(t, raw, db, secret)
	})
}

// A modified database must never crash, and anything that still opens must
// decrypt to the original secret or not at all.
func FuzzDatabase(f *testing.F) {
	hdr, db, secret := fuzzVault(f)
	f.Add(uint16(0), byte(1))
	f.Add(uint16(100), byte(0x80))
	f.Add(uint16(4096+200), byte(0xff))
	f.Fuzz(func(t *testing.T, at uint16, flip byte) {
		if flip == 0 {
			return
		}
		bad := bytes.Clone(db)
		bad[int(at)%len(bad)] ^= flip
		checkOpen(t, hdr, bad, secret)
	})
}
