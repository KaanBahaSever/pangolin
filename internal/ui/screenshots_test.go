//go:build screenshots

package ui

// Renders the screenshots used by the website and the README:
//
//	SHOT_DIR=/tmp/shots go test -tags screenshots ./internal/ui -run TestScreenshots
//	python tools/screenshots/frame.py /tmp/shots docs
//
// Each screen is captured at twice the window size, in English and Turkish,
// light and dark.

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/vault"
	"github.com/KaanBahaSever/pangolin/internal/version"
)

type demoEntry struct {
	en, tr   string // titles
	username string
	url      string
	password string
	notes    string
}

var demo = []demoEntry{
	{"GitHub", "GitHub", "octocat", "github.com", "x9$kd-Lq2!vbn#4Rt", "Recovery codes are in the safe."},
	{"Email", "E-posta", "ada@example.com", "mail.example.com", "plum-river-77", ""},
	{"Home Wi-Fi", "Ev Wi-Fi", "Pangolin-5G", "", "sunny-otter-2024", ""},
	{"Bank", "Banka", "10482231", "bank.example.com", "N7&qf2@Lm", ""},
	{"Music", "Müzik", "ada.lovelace", "music.example.com", "tune-in-41", ""},
}

func TestScreenshots(t *testing.T) {
	dir := os.Getenv("SHOT_DIR")
	if dir == "" {
		t.Skip("SHOT_DIR not set")
	}
	for _, lang := range []i18n.Lang{i18n.English, i18n.Turkish} {
		for _, mode := range []string{themeLight, themeDark} {
			shoot(t, dir, lang, mode)
		}
	}
}

func shoot(t *testing.T, dir string, lang i18n.Lang, mode string) {
	fa := test.NewApp()
	defer fa.Quit()
	fa.Preferences().SetString(prefLanguage, string(lang))
	fa.Preferences().SetString(prefTheme, mode)
	a := New(fa, t.TempDir(), testParams)
	version.Version = "1.1.0"

	capture := func(name string) {
		a.win.Resize(fyne.NewSize(980, 640))
		c := a.win.Canvas().(test.WindowlessCanvas)
		c.SetScale(2)
		c.Unfocus()
		f, err := os.Create(filepath.Join(dir, "raw-"+string(lang)+"-"+mode+"-"+name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, c.Capture()); err != nil {
			t.Fatal(err)
		}
	}

	capture("create")
	a.gate.password.SetText(testPassword)
	a.gate.confirm.SetText(testPassword)
	a.gate.submit()
	settle(t, a)
	capture("empty")

	for _, d := range demo {
		title := d.en
		if lang == i18n.Turkish {
			title = d.tr
		}
		notes := d.notes
		if lang == i18n.Turkish && notes != "" {
			notes = "Kurtarma kodları kasada."
		}
		if _, err := a.vault.Add(vault.Entry{Title: title, Username: d.username, URL: d.url,
			Password: []byte(d.password), Notes: []byte(notes)}); err != nil {
			t.Fatal(err)
		}
	}
	a.view.reload()
	var github vault.Summary
	for _, s := range a.view.items {
		if s.Title == "GitHub" {
			github = s
		}
	}
	a.view.show(github.ID)
	capture("detail")

	a.view.editEntry(github)
	capture("editor")

	a.view.showSettings()
	capture("settings")

	a.lock()
	capture("unlock")
}
