package ui

import (
	"image/color"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/session"
	"github.com/KaanBahaSever/pangolin/internal/vault"
)

var testParams = crypto.KDFParams{Time: 1, MemoryKiB: 8 * 1024, Threads: 1}

const testPassword = "correct horse battery"

// settle waits for a background key derivation to finish.
func settle(t *testing.T, a *App) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for a.busy.Load() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the key derivation")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Headless walk through the main flow: create a vault, add an entry, copy its
// password, lock, fail to unlock, unlock, edit, delete.
func TestMainFlow(t *testing.T) {
	fa := test.NewApp()
	defer fa.Quit()
	dir := t.TempDir()
	a := New(fa, dir, testParams)
	clipboard := fa.Clipboard()

	// First run shows the create screen.
	if a.gate == nil || a.gate.confirm == nil {
		t.Fatal("expected the create screen")
	}
	// The language can be switched before there is a vault, and is remembered.
	a.setLanguage(i18n.Turkish)
	if a.gate.button.Text != "Kasayı oluştur" {
		t.Fatalf("button in Turkish = %q", a.gate.button.Text)
	}
	if fa.Preferences().String(prefLanguage) != "tr" {
		t.Fatal("language choice was not saved")
	}
	a.setLanguage(i18n.English)
	if a.gate.button.Text != "Create vault" {
		t.Fatalf("button in English = %q", a.gate.button.Text)
	}
	a.gate.password.SetText("short")
	a.gate.confirm.SetText("short")
	a.gate.submit()
	if a.vault != nil || vault.Exists(dir) {
		t.Fatal("a short master password was accepted")
	}
	a.gate.password.SetText(testPassword)
	a.gate.confirm.SetText(testPassword + "x")
	a.gate.submit()
	if vault.Exists(dir) {
		t.Fatal("mismatching passwords were accepted")
	}
	g := a.gate
	g.password.SetText(testPassword)
	g.confirm.SetText(testPassword)
	g.submit()
	settle(t, a)
	if a.vault == nil || a.view == nil {
		t.Fatalf("vault was not created: %q", g.status.Text)
	}
	if g.password.Text != "" || g.confirm.Text != "" {
		t.Fatal("the master password was left in the form")
	}

	// Add an entry.
	vv := a.view
	vv.newEntry()
	vv.editor.save()
	if len(vv.items) != 0 {
		t.Fatal("an entry without a title was saved")
	}
	vv.editor.title.SetText("GitHub")
	vv.editor.username.SetText("octocat")
	vv.editor.password.SetText("hunter2-secret")
	vv.editor.notes.SetText("recovery codes")
	vv.editor.save()
	if len(vv.items) != 1 || vv.items[0].Title != "GitHub" || vv.selected != vv.items[0].ID {
		t.Fatalf("entry not saved and shown: %+v, selected %q", vv.items, vv.selected)
	}
	id := vv.items[0].ID

	// Search narrows the list and puts the shown entry away.
	vv.search.SetText("nothing like this")
	if len(vv.items) != 0 || vv.selected != "" {
		t.Fatal("search did not filter")
	}
	vv.search.SetText("OCTO")
	if len(vv.items) != 1 {
		t.Fatal("search did not match the username")
	}

	// The theme follows the system unless one is chosen, and is remembered.
	background := func(system fyne.ThemeVariant) color.Color {
		return fa.Settings().Theme().Color(theme.ColorNameBackground, system)
	}
	if background(theme.VariantLight) == background(theme.VariantDark) {
		t.Fatal("the system theme is not followed")
	}
	a.setTheme(themeLight)
	if background(theme.VariantDark) != color.Color(lightPalette.background) {
		t.Fatal("the light theme was not forced")
	}
	a.setTheme(themeDark)
	if background(theme.VariantLight) != color.Color(darkPalette.background) || fa.Preferences().String(prefTheme) != themeDark {
		t.Fatal("the dark theme was not forced and saved")
	}
	a.setTheme(themeSystem)

	// Switching language while unlocked rebuilds the screen and keeps the data.
	a.setLanguage(i18n.Turkish)
	vv = a.view
	if a.vault == nil || len(vv.items) != 1 || vv.search.PlaceHolder != "Ara" {
		t.Fatalf("after switching to Turkish: %d items, placeholder %q", len(vv.items), vv.search.PlaceHolder)
	}
	a.setLanguage(i18n.English)
	vv = a.view

	// Copy, then lock: the clipboard must be emptied and the keys gone.
	vv.copyPassword(id)
	if clipboard.Content() != "hunter2-secret" {
		t.Fatalf("clipboard = %q", clipboard.Content())
	}
	a.lock()
	if clipboard.Content() != "" {
		t.Fatal("locking did not clear the clipboard")
	}
	if a.vault != nil || a.view != nil || a.gate == nil || a.gate.confirm != nil {
		t.Fatal("expected the unlock screen after locking")
	}

	// Wrong password, then the right one.
	g = a.gate
	g.password.SetText("not the password!")
	g.submit()
	settle(t, a)
	if a.vault != nil {
		t.Fatal("unlocked with a wrong password")
	}
	if g.status.Text == "" || g.status.Text == "Unlocking…" {
		t.Fatalf("no error shown, status %q", g.status.Text)
	}
	g.password.SetText(testPassword)
	g.submit()
	settle(t, a)
	if a.vault == nil {
		t.Fatalf("could not unlock: %q", g.status.Text)
	}

	// Edit: the form is filled from the vault, and the change is stored.
	vv = a.view
	if len(vv.items) != 1 {
		t.Fatal("entry lost across lock and unlock")
	}
	vv.editEntry(vv.items[0])
	if vv.editor.password.Text != "hunter2-secret" || vv.editor.notes.Text != "recovery codes" {
		t.Fatal("editor was not filled from the vault")
	}
	vv.editor.password.SetText("changed")
	vv.editor.save()
	pw, err := a.vault.Secret(id, vault.FieldPassword)
	if err != nil || string(pw) != "changed" {
		t.Fatalf("password after edit = %q, %v", pw, err)
	}

	if err := a.vault.Delete(id); err != nil {
		t.Fatal(err)
	}
	vv.reload()
	if len(vv.items) != 0 {
		t.Fatal("entry not deleted")
	}
	a.lock()
}

func TestIdleLocks(t *testing.T) {
	fa := test.NewApp()
	defer fa.Quit()
	a := New(fa, t.TempDir(), testParams)
	i18n.Set(i18n.English)
	a.gate.password.SetText(testPassword)
	a.gate.confirm.SetText(testPassword)
	a.gate.submit()
	settle(t, a)
	if a.vault == nil {
		t.Fatal("vault was not created")
	}

	// Same wiring as New, plus a signal the test can wait on.
	a.idle.Stop()
	done := make(chan struct{})
	a.idle = session.NewIdleTimer(func() {
		fyne.DoAndWait(a.lock)
		close(done)
	})
	a.idle.Start(50 * time.Millisecond)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the idle timer did not fire")
	}
	if a.vault != nil || a.gate == nil {
		t.Fatal("the vault did not lock when idle")
	}
}
