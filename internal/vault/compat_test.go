package vault

import (
	"os"
	"path/filepath"
	"testing"
)

// testdata/v1 holds a vault written by Pangolin 1.0. Every later version must
// still open it: users upgrade by simply starting the new version.
//
// The fixture is only ever regenerated on purpose, with
//
//	PANGOLIN_WRITE_FIXTURE=1 go test ./internal/vault -run TestFormatV1
//
// and never when the format changes; a format change needs a migration and a
// new fixture beside this one.
const (
	fixtureDir      = "testdata/v1"
	fixturePassword = "pangolin compat v1"
)

var fixtureEntries = []Entry{
	{Title: "GitHub", Username: "octocat", URL: "github.com", Password: []byte("x9$kd-Lq2!vbn"), Notes: []byte("recovery codes")},
	{Title: "İstanbul Kart", Username: "ışık", Password: []byte("çğıöşü")},
}

func TestFormatV1(t *testing.T) {
	if os.Getenv("PANGOLIN_WRITE_FIXTURE") == "1" {
		os.RemoveAll(fixtureDir)
		v, err := Create(fixtureDir, []byte(fixturePassword), testParams)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range fixtureEntries {
			mustAdd(t, v, e)
		}
		v.Close()
	}

	// Work on a copy, so the test never modifies the fixture.
	dir := t.TempDir()
	for _, name := range []string{headerFile, dbFile} {
		raw, err := os.ReadFile(filepath.Join(fixtureDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	v, err := Open(dir, []byte(fixturePassword))
	if err != nil {
		t.Fatalf("a 1.0 vault no longer opens: %v", err)
	}
	defer v.Close()
	list, err := v.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != len(fixtureEntries) {
		t.Fatalf("%d entries, want %d", len(list), len(fixtureEntries))
	}
	for _, want := range fixtureEntries {
		found := false
		for _, s := range list {
			if s.Title != want.Title {
				continue
			}
			found = true
			if s.Username != want.Username || s.URL != want.URL {
				t.Errorf("%s: got %+v", want.Title, s)
			}
			for field, plain := range map[Field][]byte{FieldPassword: want.Password, FieldNotes: want.Notes} {
				got, err := v.Secret(s.ID, field)
				if err != nil || string(got) != string(plain) {
					t.Errorf("%s %s = %q, %v", want.Title, field, got, err)
				}
			}
		}
		if !found {
			t.Errorf("entry %q is missing", want.Title)
		}
	}
	// The vault stays usable after the upgrade.
	if _, err := v.Add(Entry{Title: "added by a newer version"}); err != nil {
		t.Fatal(err)
	}
}
