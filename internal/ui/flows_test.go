package ui

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/vault"
	"github.com/KaanBahaSever/pangolin/internal/version"
)

// Tests in this file drive the screens the way a user would: they find
// buttons by their text and tap them.

// all returns every object of type T in the tree under o.
func all[T fyne.CanvasObject](o fyne.CanvasObject) []T {
	var out []T
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		if o == nil {
			return
		}
		if v, ok := o.(T); ok {
			out = append(out, v)
		}
		switch c := o.(type) {
		case *fyne.Container:
			for _, child := range c.Objects {
				walk(child)
			}
		case fyne.Widget:
			for _, child := range test.WidgetRenderer(c).Objects() {
				walk(child)
			}
		}
	}
	walk(o)
	return out
}

// screen is the window content plus any dialogs above it.
func screen(a *App) []fyne.CanvasObject {
	return append([]fyne.CanvasObject{a.win.Content()}, a.win.Canvas().Overlays().List()...)
}

func findButton(t *testing.T, a *App, text string) *widget.Button {
	t.Helper()
	for _, root := range screen(a) {
		for _, b := range all[*widget.Button](root) {
			if b.Text == text {
				return b
			}
		}
	}
	t.Fatalf("no button %q on screen", text)
	return nil
}

func screenHasText(a *App, text string) bool {
	for _, root := range screen(a) {
		for _, l := range all[*widget.Label](root) {
			if strings.Contains(l.Text, text) {
				return true
			}
		}
	}
	return false
}

// unlocked returns an app with a fresh vault holding one entry.
func unlocked(t *testing.T) (*App, fyne.App, vault.Summary) {
	t.Helper()
	fa := test.NewApp()
	t.Cleanup(fa.Quit)
	a := New(fa, t.TempDir(), testParams)
	a.setLanguage(i18n.English)
	a.gate.password.SetText(testPassword)
	a.gate.confirm.SetText(testPassword)
	a.gate.submit()
	settle(t, a)
	if a.vault == nil {
		t.Fatal("vault not created")
	}
	if _, err := a.vault.Add(vault.Entry{Title: "GitHub", Username: "octocat", URL: "github.com",
		Password: []byte("hunter2-secret"), Notes: []byte("codes")}); err != nil {
		t.Fatal(err)
	}
	a.view.reload()
	t.Cleanup(func() { a.lock() })
	return a, fa, a.view.items[0]
}

func TestSettingsControls(t *testing.T) {
	a, fa, _ := unlocked(t)
	a.view.showSettings()

	// Drop-downs write their preference.
	selects := all[*widget.Select](a.win.Content())
	if len(selects) != 2 {
		t.Fatalf("%d drop-downs on the settings page, want 2", len(selects))
	}
	selects[0].SetSelected(i18n.T("After 1 minute"))
	selects[1].SetSelected(i18n.T("After 60 seconds"))
	if fa.Preferences().Int(prefAutoLockMinutes) != 1 || fa.Preferences().Int(prefClipboardSeconds) != 60 {
		t.Fatalf("preferences: lock %d, clipboard %d", fa.Preferences().Int(prefAutoLockMinutes), fa.Preferences().Int(prefClipboardSeconds))
	}
	if a.autoLock().Minutes() != 1 || a.clipboardTTL().Seconds() != 60 {
		t.Fatal("the app does not use the new delays")
	}

	// Language buttons rebuild the screen and stay on the settings page.
	test.Tap(findButton(t, a, "Türkçe"))
	if a.view.page != pageSettings || fa.Preferences().String(prefLanguage) != "tr" || !screenHasText(a, "Görünüm") {
		t.Fatal("switching to Turkish left the settings page or did not translate it")
	}
	test.Tap(findButton(t, a, "English"))
	if !screenHasText(a, "Appearance") {
		t.Fatal("switching back to English did not translate the page")
	}

	// Theme buttons, last: Fyne's test app applies a theme on a goroutine of
	// its own, so the screen is not inspected after this point.
	themeButtons := map[string]*widget.Button{}
	for _, label := range []string{"Dark", "Light", "System"} {
		themeButtons[label] = findButton(t, a, label)
	}
	for _, c := range []struct{ label, mode string }{{"Dark", themeDark}, {"Light", themeLight}, {"System", themeSystem}} {
		test.Tap(themeButtons[c.label])
		if got := fa.Preferences().String(prefTheme); got != c.mode {
			t.Fatalf("after %q the theme is %q", c.label, got)
		}
	}
}

func TestChangePasswordThroughSettings(t *testing.T) {
	a, _, _ := unlocked(t)
	a.spawn = func(f func()) { f() }
	a.view.showSettings()

	fill := func(current, next, confirm string) {
		t.Helper()
		// The form stays open after a validation error; open it if closed.
		for _, b := range all[*widget.Button](a.win.Content()) {
			if b.Text == i18n.T("Change") {
				test.Tap(b)
			}
		}
		var fields []*widget.Entry
		for _, e := range all[*widget.Entry](a.win.Content()) {
			if e.Password && e.PlaceHolder != "" {
				fields = append(fields, e)
			}
		}
		if len(fields) != 3 {
			t.Fatalf("%d password fields, want 3", len(fields))
		}
		fields[0].SetText(current)
		fields[1].SetText(next)
		fields[2].SetText(confirm)
		test.Tap(findButton(t, a, i18n.T("Change password")))
		settle(t, a)
	}

	fill(testPassword, "short", "short")
	if !strings.Contains(a.view.status.Text, "12") {
		t.Fatalf("no length warning, status %q", a.view.status.Text)
	}
	fill(testPassword, "a long new password", "a long new passwort")
	if a.view.status.Text != i18n.T("The two new passwords do not match.") {
		t.Fatalf("no mismatch warning, status %q", a.view.status.Text)
	}
	a.closeDialogs()
	a.view.showSettings()
	fill("not the current one", "a long new password", "a long new password")
	if !screenHasText(a, i18n.T("The current password is not correct.")) {
		t.Fatal("a wrong current password was not reported")
	}
	a.closeDialogs()
	a.view.showSettings()
	fill(testPassword, "a long new password", "a long new password")
	if !screenHasText(a, i18n.T("The master password has been changed.")) {
		t.Fatal("no confirmation after changing the password")
	}

	a.lock()
	a.gate.password.SetText(testPassword)
	a.gate.submit()
	settle(t, a)
	if a.vault != nil {
		t.Fatal("the old password still unlocks")
	}
	a.gate.password.SetText("a long new password")
	a.gate.submit()
	settle(t, a)
	if a.vault == nil || len(a.view.items) != 1 {
		t.Fatal("the new password does not unlock, or the entry is gone")
	}
}

func TestGeneratorFillsPassword(t *testing.T) {
	a, _, _ := unlocked(t)
	a.view.newEntry()
	test.Tap(findButton(t, a, i18n.T("Generate")))
	sliders := all[*widget.Slider](a.win.Canvas().Overlays().Top())
	if len(sliders) != 1 {
		t.Fatalf("%d sliders in the generator", len(sliders))
	}
	sliders[0].SetValue(32)
	test.Tap(findButton(t, a, i18n.T("Use")))
	if got := a.view.editor.password.Text; len(got) != 32 {
		t.Fatalf("generated password %q has %d characters, want 32", got, len(got))
	}
}

func TestDeleteThroughConfirmation(t *testing.T) {
	a, _, s := unlocked(t)
	a.view.show(s.ID)
	var remove *widget.Button
	for _, b := range all[*widget.Button](a.win.Content()) {
		if b.Importance == widget.DangerImportance {
			remove = b
		}
	}
	if remove == nil {
		t.Fatal("no delete button")
	}

	test.Tap(remove)
	test.Tap(findButton(t, a, i18n.T("Cancel")))
	if len(a.view.items) != 1 {
		t.Fatal("cancelling deleted the entry")
	}

	test.Tap(remove)
	// The dialog's confirm button reads "Delete", like nothing else on screen.
	test.Tap(findButton(t, a, i18n.T("Delete")))
	if len(a.view.items) != 0 || a.view.page != pageEmpty {
		t.Fatal("the entry was not deleted")
	}
	if _, err := a.vault.Secret(s.ID, vault.FieldPassword); !errors.Is(err, vault.ErrEntryNotFound) {
		t.Fatalf("the entry is still in the vault: %v", err)
	}
}

func TestDetailActions(t *testing.T) {
	a, fa, s := unlocked(t)
	a.view.show(s.ID)

	// The first copy button belongs to the username.
	var copies, reveals []*widget.Button
	for _, b := range all[*widget.Button](a.win.Content()) {
		if b.Icon == nil {
			continue
		}
		switch b.Icon.Name() {
		case theme.ContentCopyIcon().Name():
			copies = append(copies, b)
		case theme.VisibilityIcon().Name():
			reveals = append(reveals, b)
		}
	}
	if len(copies) != 3 || len(reveals) != 2 {
		t.Fatalf("%d copy and %d show buttons, want 3 and 2", len(copies), len(reveals))
	}
	test.Tap(copies[0])
	if fa.Clipboard().Content() != "octocat" {
		t.Fatalf("clipboard = %q", fa.Clipboard().Content())
	}
	test.Tap(copies[1])
	if fa.Clipboard().Content() != "hunter2-secret" {
		t.Fatalf("clipboard = %q", fa.Clipboard().Content())
	}

	// Show and hide the password.
	test.Tap(reveals[0])
	if !screenHasText(a, "hunter2-secret") {
		t.Fatal("the password was not shown")
	}
	test.Tap(reveals[0])
	if screenHasText(a, "hunter2-secret") {
		t.Fatal("the password was not hidden again")
	}
	// Selecting another page masks it as well.
	test.Tap(reveals[0])
	a.view.showSettings()
	a.view.show(s.ID)
	if screenHasText(a, "hunter2-secret") {
		t.Fatal("the password stayed visible after leaving the entry")
	}
}

func TestUpdateCheck(t *testing.T) {
	a, _, _ := unlocked(t)
	a.spawn = func(f func()) { f() }
	old := version.Version
	defer func() { version.Version = old }()

	for _, c := range []struct {
		name, current string
		status        int
		body, want    string
	}{
		{"newer", "1.3.0", 200, `{"tag_name":"v1.4.0"}`, "Pangolin 1.4.0 is available."},
		{"same", "1.4.0", 200, `{"tag_name":"v1.4.0"}`, "You have the latest version."},
		{"dev", "dev", 200, `{"tag_name":"v1.4.0"}`, "This is a development build."},
		{"error", "1.3.0", 500, `oops`, "Could not check for updates. Are you online?"},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				fmt.Fprint(w, c.body)
			}))
			defer srv.Close()
			a.updateURL = srv.URL
			version.Version = c.current
			a.view.showSettings()
			test.Tap(findButton(t, a, i18n.T("Check")))
			if !screenHasText(a, c.want) {
				t.Fatalf("status does not say %q", c.want)
			}
			hasDownload := false
			for _, b := range all[*widget.Button](a.win.Content()) {
				hasDownload = hasDownload || b.Text == i18n.T("Download")
			}
			if hasDownload != (c.name == "newer") {
				t.Fatalf("download button shown: %v", hasDownload)
			}
		})
	}
}

func TestGateMessages(t *testing.T) {
	i18n.Set(i18n.English)
	for err, want := range map[error]string{
		fmt.Errorf("x: %w", vault.ErrUnlock):          "Wrong master password, or the vault is damaged.",
		fmt.Errorf("x: %w", vault.ErrMissingDatabase): "The vault's database file (vault.db) is missing.",
		vault.ErrWeakPassword:                         "Use at least 12 characters.",
		errors.New("disk on fire"):                    "The vault could not be opened: disk on fire",
	} {
		if got := gateMessage(err); got != want {
			t.Errorf("gateMessage(%v) = %q, want %q", err, got, want)
		}
	}
}
