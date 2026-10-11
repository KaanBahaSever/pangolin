package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	raw, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Two vaults with the same password must not open each other's database.
func TestHeaderFromAnotherVault(t *testing.T) {
	a, dirA := newVault(t)
	mustAdd(t, a, Entry{Title: "a"})
	a.Close()
	b, dirB := newVault(t)
	b.Close()

	mixed := t.TempDir()
	copyFile(t, filepath.Join(dirB, headerFile), filepath.Join(mixed, headerFile))
	copyFile(t, filepath.Join(dirA, dbFile), filepath.Join(mixed, dbFile))
	if _, err := Open(mixed, testPass); !errors.Is(err, ErrUnlock) {
		t.Fatalf("mismatched header and database: err = %v", err)
	}
}

// Two windows of Pangolin on the same vault: both must be able to write,
// and each must see the other's entries.
func TestTwoHandlesOnOneVault(t *testing.T) {
	v1, dir := newVault(t)
	v2, err := Open(dir, testPass)
	if err != nil {
		t.Fatal(err)
	}
	defer v2.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for _, v := range []*Vault{v1, v2} {
		wg.Add(1)
		go func(v *Vault) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				if _, err := v.Add(Entry{Title: "concurrent", Password: []byte("p")}); err != nil {
					errs <- err
				}
			}
		}(v)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent write failed: %v", err)
	}
	for _, v := range []*Vault{v1, v2} {
		if list, _ := v.List(""); len(list) != 50 {
			t.Errorf("a handle sees %d entries, want 50", len(list))
		}
	}

	// Changing the password in one window leaves the other working.
	next := []byte("changed in the other window")
	if err := v1.ChangePassword(testPass, next); err != nil {
		t.Fatal(err)
	}
	if _, err := v2.Add(Entry{Title: "after the change"}); err != nil {
		t.Fatalf("the other window stopped working: %v", err)
	}
	v1.Close()
	v2.Close()
	if _, err := Open(dir, testPass); !errors.Is(err, ErrUnlock) {
		t.Fatal("the old password still opens the vault")
	}
	v3, err := Open(dir, next)
	if err != nil {
		t.Fatal(err)
	}
	defer v3.Close()
	if list, _ := v3.List(""); len(list) != 51 {
		t.Fatalf("%d entries after reopening, want 51", len(list))
	}
}

// Unusual and large values must come back byte for byte.
func TestUnusualValuesRoundTrip(t *testing.T) {
	v, dir := newVault(t)
	big := bytes.Repeat([]byte("notes with ünïcödé 🦔 "), 50_000) // about 1.2 MB
	cases := []Entry{
		{Title: `'); DROP TABLE entries;--`, Username: `" OR 1=1 --`, URL: "javascript:alert(1)", Password: []byte(`%s%n\x00`)},
		{Title: "🦔 Kirpi 🔐", Username: "ünlü@örnek.com.tr", Password: []byte("🔑🔑🔑"), Notes: []byte("emoji")},
		{Title: "مرحبا שלום", Username: "rtl", Password: []byte("rtl")},
		{Title: "binary", Password: []byte{0, 1, 2, 255, 0, 254}, Notes: []byte{0}},
		{Title: strings.Repeat("Long title ", 1000), Password: []byte(strings.Repeat("p", 10_000)), Notes: big},
	}
	ids := make([]string, len(cases))
	for i, e := range cases {
		ids[i] = mustAdd(t, v, e)
	}
	v.Close()

	v, err := Open(dir, testPass)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	list, _ := v.List("")
	if len(list) != len(cases) {
		t.Fatalf("%d entries, want %d", len(list), len(cases))
	}
	for i, e := range cases {
		pw, err := v.Secret(ids[i], FieldPassword)
		if err != nil || !bytes.Equal(pw, e.Password) {
			t.Errorf("entry %d password: %q, %v", i, pw, err)
		}
		notes, err := v.Secret(ids[i], FieldNotes)
		if err != nil || !bytes.Equal(notes, e.Notes) {
			t.Errorf("entry %d notes differ (%d bytes, want %d), %v", i, len(notes), len(e.Notes), err)
		}
	}
	if got, _ := v.List("kirpi"); len(got) != 1 {
		t.Error("emoji title not found by search")
	}
	if got, _ := v.List("DROP TABLE"); len(got) != 1 {
		t.Error("SQL-like title not found by search")
	}
}

// Folders with spaces and Turkish letters, as Windows user names often have.
func TestVaultInFolderWithTurkishName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Kullanıcılar", "Şükrü Çağlar", "Belgeler & Yedek", "Pangolin")
	v, err := Create(dir, testPass, testParams)
	if err != nil {
		t.Fatal(err)
	}
	mustAdd(t, v, Entry{Title: "ok", Password: []byte("p")})
	v.Close()
	v, err = Open(dir, testPass)
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	if list, _ := v.List(""); len(list) != 1 {
		t.Fatal("entry lost")
	}
}

// Searching stays quick in a large vault: it runs on every keystroke.
func TestLargeVaultStaysQuick(t *testing.T) {
	if testing.Short() {
		t.Skip("slow")
	}
	v, _ := newVault(t)
	tx := 3000
	for i := 0; i < tx; i++ {
		mustAdd(t, v, Entry{Title: "Entry " + strings.Repeat("x", i%40), Username: "user", URL: "example.com", Password: []byte("p")})
	}
	start := time.Now()
	list, err := v.List("")
	if err != nil || len(list) != tx {
		t.Fatalf("%d entries, %v", len(list), err)
	}
	all := time.Since(start)
	start = time.Now()
	v.List("xxxxxxxxxxxx")
	search := time.Since(start)
	t.Logf("list %d entries: %v, search: %v", tx, all, search)
	if all > time.Second || search > time.Second {
		t.Fatalf("too slow: list %v, search %v", all, search)
	}
}

// A crash while replacing the header leaves a temporary file behind; it
// must not get in the way.
func TestLeftoverTemporaryHeader(t *testing.T) {
	v, dir := newVault(t)
	v.Close()
	if err := os.WriteFile(filepath.Join(dir, headerFile+".123.tmp"), []byte("half written"), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := Open(dir, testPass)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ChangePassword(testPass, []byte("a different long password")); err != nil {
		t.Fatal(err)
	}
	v.Close()
}

// The same wrong password must keep failing; a correct one must keep working.
func TestManyUnlockAttempts(t *testing.T) {
	v, dir := newVault(t)
	v.Close()
	for i := 0; i < 20; i++ {
		if _, err := Open(dir, []byte("wrong password number "+strings.Repeat("!", i))); !errors.Is(err, ErrUnlock) {
			t.Fatalf("attempt %d: err = %v", i, err)
		}
	}
	v, err := Open(dir, testPass)
	if err != nil {
		t.Fatal(err)
	}
	v.Close()
}
