package vault

import (
	"os"
	"path/filepath"
	"testing"
)

// testdata/releases/<tag> holds a vault written by that release's own code.
// Every later version must open all of them unchanged: users upgrade by
// replacing the program and starting it again.
//
// After a release, add its vault the same way: check the tag out in a
// separate worktree, copy this file's sibling generator
// (tools/fixture/zz_release_fixture_test.go.txt, renamed to .go) into its internal/vault, run
//
//	RELEASE_FIXTURE_DIR=<dir> go test ./internal/vault -run TestWriteReleaseFixture
//
// and copy vault.header and vault.db into testdata/releases/<tag>. Never
// regenerate an existing fixture: a format change needs a migration instead.
const releaseFixturePassword = "pangolin sürüm testi ğüşıöç"

var releaseFixtureEntries = []Entry{
	{Title: "GitHub", Username: "octocat", URL: "github.com", Password: []byte("x9$kd-Lq2!vbn"), Notes: []byte("recovery codes")},
	{Title: "İstanbul Kart", Username: "ışık", Password: []byte("çğıöşü-Parola")},
	{Title: "Empty fields"},
}

func TestEveryReleaseOpens(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join("testdata", "releases", "v*"))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, d := range dirs {
		found[filepath.Base(d)] = true
	}
	// The first public release must always be among them.
	if !found["v0.1.0"] {
		t.Fatalf("the v0.1.0 fixture is missing; found %v", dirs)
	}
	for _, src := range dirs {
		t.Run(filepath.Base(src), func(t *testing.T) { checkReleaseFixture(t, src) })
	}
}

func checkReleaseFixture(t *testing.T, src string) {
	// Work on a copy, so the test never modifies the fixture.
	dir := t.TempDir()
	for _, name := range []string{headerFile, dbFile} {
		raw, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	v, err := Open(dir, []byte(releaseFixturePassword))
	if err != nil {
		t.Fatalf("does not open: %v", err)
	}
	list, err := v.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(releaseFixtureEntries) {
		t.Fatalf("%d entries, want %d", len(list), len(releaseFixtureEntries))
	}
	for _, want := range releaseFixtureEntries {
		var got *Summary
		for i := range list {
			if list[i].Title == want.Title {
				got = &list[i]
			}
		}
		if got == nil {
			t.Errorf("entry %q is missing", want.Title)
			continue
		}
		if got.Username != want.Username || got.URL != want.URL {
			t.Errorf("%s: got %+v", want.Title, *got)
		}
		for field, plain := range map[Field][]byte{FieldPassword: want.Password, FieldNotes: want.Notes} {
			secret, err := v.Secret(got.ID, field)
			if err != nil || string(secret) != string(plain) {
				t.Errorf("%s %s = %q, %v", want.Title, field, secret, err)
			}
		}
	}

	// After the upgrade the vault keeps working: entries can be added and
	// the master password changed, and it opens again afterwards.
	if _, err := v.Add(Entry{Title: "added after the upgrade", Password: []byte("new")}); err != nil {
		t.Fatal(err)
	}
	next := []byte("a new master password after upgrading")
	if err := v.ChangePassword([]byte(releaseFixturePassword), next); err != nil {
		t.Fatal(err)
	}
	v.Close()
	v, err = Open(dir, next)
	if err != nil {
		t.Fatalf("does not reopen after the upgrade: %v", err)
	}
	defer v.Close()
	if list, _ := v.List(""); len(list) != len(releaseFixtureEntries)+1 {
		t.Fatalf("%d entries after reopening", len(list))
	}
}
