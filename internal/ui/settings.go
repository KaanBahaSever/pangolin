package ui

import (
	"errors"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/vault"
	"github.com/KaanBahaSever/pangolin/internal/version"
)

type choice struct {
	label string
	value int
}

var (
	autoLockChoices = []choice{
		{"After 1 minute", 1}, {"After 5 minutes", 5}, {"After 15 minutes", 15},
		{"After 30 minutes", 30}, {"Never", 0},
	}
	clipboardChoices = []choice{
		{"After 10 seconds", 10}, {"After 20 seconds", 20}, {"After 30 seconds", 30},
		{"After 60 seconds", 60},
	}
)

// choiceSelect builds a drop-down bound to an integer preference.
func (a *App) choiceSelect(choices []choice, key string, fallback int, changed func()) *widget.Select {
	labels := make([]string, len(choices))
	current := a.fyne.Preferences().IntWithFallback(key, fallback)
	selected := ""
	for i, c := range choices {
		labels[i] = i18n.T(c.label)
		if c.value == current {
			selected = labels[i]
		}
	}
	s := widget.NewSelect(labels, nil)
	s.Selected = selected
	s.OnChanged = func(label string) {
		a.touch()
		for _, c := range choices {
			if i18n.T(c.label) == label {
				a.fyne.Preferences().SetInt(key, c.value)
			}
		}
		if changed != nil {
			changed()
		}
	}
	return s
}

func (a *App) showSettings() {
	a.touch()
	autoLock := a.choiceSelect(autoLockChoices, prefAutoLockMinutes, defaultAutoLockMinutes,
		func() { a.idle.Start(a.autoLock()) })
	clipboard := a.choiceSelect(clipboardChoices, prefClipboardSeconds, defaultClipboardSeconds, nil)

	var d dialog.Dialog
	change := widget.NewButton(i18n.T("Change master password…"), func() {
		d.Hide()
		a.showChangePassword()
	})
	form := widget.NewForm(
		widget.NewFormItem(i18n.T("Language"), a.languageSelect()),
		widget.NewFormItem(i18n.T("Theme"), a.themeSelect()),
		widget.NewFormItem(i18n.T("Lock when idle"), autoLock),
		widget.NewFormItem(i18n.T("Clear clipboard"), clipboard),
		widget.NewFormItem("", change),
	)
	about := widget.NewLabel("Pangolin " + version.Version)
	about.Alignment = fyne.TextAlignCenter
	about.Importance = widget.LowImportance
	d = dialog.NewCustom(i18n.T("Settings"), i18n.T("Close"), container.NewPadded(container.NewVBox(form, about)), a.win)
	d.Resize(fyne.NewSize(420, 0))
	d.Show()
}

func (a *App) showChangePassword() {
	title := i18n.T("Change master password")
	current := widget.NewPasswordEntry()
	next := widget.NewPasswordEntry()
	confirm := widget.NewPasswordEntry()
	items := []*widget.FormItem{
		widget.NewFormItem(i18n.T("Current"), current),
		widget.NewFormItem(i18n.T("New"), next),
		widget.NewFormItem(i18n.T("Repeat new"), confirm),
	}
	d := dialog.NewForm(title, i18n.T("Change"), i18n.T("Cancel"), items, func(ok bool) {
		a.touch()
		if !ok {
			return
		}
		switch {
		case utf8.RuneCountInString(next.Text) < vault.MinPasswordLength:
			a.showInfo(title, i18n.Tf("Use at least %d characters.", vault.MinPasswordLength))
			return
		case next.Text != confirm.Text:
			a.showInfo(title, i18n.T("The two new passwords do not match."))
			return
		}
		a.changePassword([]byte(current.Text), []byte(next.Text))
		current.SetText("")
		next.SetText("")
		confirm.SetText("")
	}, a.win)
	d.Resize(fyne.NewSize(420, 0))
	d.Show()
}

// changePassword runs the two key derivations off the UI thread. It takes
// ownership of both slices and wipes them.
func (a *App) changePassword(current, next []byte) {
	v := a.vault
	if v == nil || !a.busy.CompareAndSwap(false, true) {
		crypto.Wipe(current)
		crypto.Wipe(next)
		return
	}
	title := i18n.T("Change master password")
	progress := dialog.NewCustomWithoutButtons(title,
		widget.NewProgressBarInfinite(), a.win)
	progress.Show()

	go func() {
		err := v.ChangePassword(current, next)
		crypto.Wipe(current)
		crypto.Wipe(next)
		fyne.Do(func() {
			defer a.busy.Store(false)
			progress.Hide()
			switch {
			case errors.Is(err, vault.ErrLocked):
				// The vault locked itself meanwhile; the unlock screen is showing.
			case errors.Is(err, vault.ErrUnlock):
				a.showInfo(title, i18n.T("The current password is not correct."))
			case err != nil:
				a.showError(err)
			default:
				a.showInfo(title, i18n.T("The master password has been changed."))
			}
		})
	}()
}

// languageSelect builds the language drop-down used on the first screen and
// in the settings.
func (a *App) languageSelect() *widget.Select {
	names := make([]string, len(i18n.Languages))
	for i, l := range i18n.Languages {
		names[i] = l.Name()
	}
	s := widget.NewSelect(names, nil)
	s.Selected = i18n.Current().Name()
	s.OnChanged = func(name string) {
		for _, l := range i18n.Languages {
			if l.Name() == name && l != i18n.Current() {
				a.setLanguage(l)
			}
		}
	}
	return s
}

// themeSelect builds the drop-down for the light, dark or system theme.
func (a *App) themeSelect() *widget.Select {
	modes := []string{themeSystem, themeLight, themeDark}
	labels := []string{i18n.T("Same as system"), i18n.T("Light"), i18n.T("Dark")}
	s := widget.NewSelect(labels, nil)
	current := a.fyne.Preferences().StringWithFallback(prefTheme, themeSystem)
	for i, mode := range modes {
		if mode == current {
			s.Selected = labels[i]
		}
	}
	s.OnChanged = func(label string) {
		for i := range labels {
			if labels[i] == label {
				a.setTheme(modes[i])
			}
		}
	}
	return s
}
