package ui

import (
	"context"
	"errors"
	"net/url"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/update"
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
	themeModes = []string{themeSystem, themeLight, themeDark}
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

func languageNames() []string {
	names := make([]string, len(i18n.Languages))
	for i, l := range i18n.Languages {
		names[i] = l.Name()
	}
	return names
}

func languageIndex() int {
	for i, l := range i18n.Languages {
		if l == i18n.Current() {
			return i
		}
	}
	return 0
}

// wide gives a control a fixed width, so a drop-down shows its whole text.
func wide(o fyne.CanvasObject, width float32) fyne.CanvasObject {
	return container.New(layout.NewGridWrapLayout(fyne.NewSize(width, o.MinSize().Height)), o)
}

// setting is one row of a settings card: a name and a short explanation on
// the left, the control on the right.
func setting(name, description string, control fyne.CanvasObject) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	text := container.New(layout.NewCustomPaddedVBoxLayout(-theme.Padding()*3), title)
	if description != "" {
		d := muted(description)
		d.SizeName = theme.SizeNameCaptionText
		d.Wrapping = fyne.TextWrapWord
		text.Add(d)
	}
	return container.NewBorder(nil, nil, nil, container.NewCenter(control), text)
}

// section is a caption followed by a card.
func section(name string, rows ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(caption(name), cardRows(rows...))
}

func (vv *vaultView) showSettings() {
	a := vv.a
	a.touch()
	vv.list.UnselectAll()

	themeIndex := 0
	current := a.fyne.Preferences().StringWithFallback(prefTheme, themeSystem)
	for i, mode := range themeModes {
		if mode == current {
			themeIndex = i
		}
	}
	appearance := section(i18n.T("Appearance"),
		setting(i18n.T("Language"), "",
			segmented(languageNames(), languageIndex(), func(i int) { a.setLanguage(i18n.Languages[i]) })),
		setting(i18n.T("Theme"), "",
			segmented([]string{i18n.T("System"), i18n.T("Light"), i18n.T("Dark")}, themeIndex,
				func(i int) { a.setTheme(themeModes[i]) })),
	)

	autoLock := a.choiceSelect(autoLockChoices, prefAutoLockMinutes, defaultAutoLockMinutes,
		func() { a.idle.Start(a.autoLock()) })
	clipboard := a.choiceSelect(clipboardChoices, prefClipboardSeconds, defaultClipboardSeconds, nil)

	// Changing the master password opens a small form inside the card.
	passwordRow := container.NewVBox()
	var showChange func()
	collapsed := func() {
		change := widget.NewButton(i18n.T("Change"), func() { showChange() })
		passwordRow.Objects = []fyne.CanvasObject{setting(i18n.T("Master password"),
			i18n.T("Unlocks the vault. It cannot be recovered."), change)}
		passwordRow.Refresh()
	}
	showChange = func() {
		a.touch()
		current, next, confirm := widget.NewPasswordEntry(), widget.NewPasswordEntry(), widget.NewPasswordEntry()
		current.SetPlaceHolder(i18n.T("Current password"))
		next.SetPlaceHolder(i18n.T("New password"))
		confirm.SetPlaceHolder(i18n.T("Repeat new password"))
		cancel := widget.NewButton(i18n.T("Cancel"), collapsed)
		cancel.Importance = widget.LowImportance
		submit := widget.NewButton(i18n.T("Change password"), nil)
		submit.Importance = widget.HighImportance
		submit.OnTapped = func() {
			a.touch()
			switch {
			case utf8.RuneCountInString(next.Text) < vault.MinPasswordLength:
				vv.say(i18n.Tf("Use at least %d characters.", vault.MinPasswordLength))
				return
			case next.Text != confirm.Text:
				vv.say(i18n.T("The two new passwords do not match."))
				return
			}
			a.changePassword([]byte(current.Text), []byte(next.Text), collapsed)
			current.SetText("")
			next.SetText("")
			confirm.SetText("")
		}
		passwordRow.Objects = []fyne.CanvasObject{container.NewVBox(
			widget.NewLabelWithStyle(i18n.T("Master password"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			current, next, confirm,
			container.NewHBox(layout.NewSpacer(), cancel, submit),
		)}
		passwordRow.Refresh()
		a.win.Canvas().Focus(current)
	}
	collapsed()

	security := section(i18n.T("Security"),
		setting(i18n.T("Lock when idle"), i18n.T("Locks the vault when you have not used it for a while."), wide(autoLock, 190)),
		setting(i18n.T("Clear clipboard"), i18n.T("Removes a copied password from the clipboard."), wide(clipboard, 190)),
		passwordRow,
	)

	folder := widget.NewLabel(a.dir)
	folder.Selectable = true
	folder.Wrapping = fyne.TextWrapBreak
	name := widget.NewLabelWithStyle(i18n.Tf("Pangolin %s", version.Version), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	license := muted(i18n.T("Free and open source, MIT License."))
	license.SizeName = theme.SizeNameCaptionText
	about := section(i18n.T("About"),
		container.NewBorder(nil, nil, logoImage(56), nil,
			container.New(layout.NewCustomPaddedVBoxLayout(-theme.Padding()*3), name, license)),
		container.NewVBox(caption(i18n.T("Vault folder")), folder),
		a.updateRow(),
	)

	vv.setPage(pageSettings, scrollPage(heading(i18n.T("Settings")), appearance, security, about))
}

// changePassword runs the two key derivations off the UI thread. It takes
// ownership of both slices and wipes them. done runs after a success.
func (a *App) changePassword(current, next []byte, done func()) {
	v := a.vault
	if v == nil || !a.busy.CompareAndSwap(false, true) {
		crypto.Wipe(current)
		crypto.Wipe(next)
		return
	}
	title := i18n.T("Change master password")
	progress := dialog.NewCustomWithoutButtons(title, widget.NewProgressBarInfinite(), a.win)
	progress.Show()

	a.spawn(func() {
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
				done()
				a.showInfo(title, i18n.T("The master password has been changed."))
			}
		})
	})
}

// updateRow is the "Updates" row of the About card. Pressing Check is the
// only way Pangolin ever goes online.
func (a *App) updateRow() fyne.CanvasObject {
	status := muted(i18n.T("Pangolin connects to GitHub only when you press Check."))
	status.SizeName = theme.SizeNameCaptionText
	status.Wrapping = fyne.TextWrapWord
	title := widget.NewLabelWithStyle(i18n.T("Updates"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	actions := container.NewHBox()
	var check *widget.Button
	check = widget.NewButton(i18n.T("Check"), func() {
		a.touch()
		check.Disable()
		status.SetText(i18n.T("Checking…"))
		current := version.Version
		a.spawn(func() {
			latest, err := update.Latest(context.Background(), a.updateURL, current)
			fyne.Do(func() {
				check.Enable()
				switch {
				case err != nil:
					status.SetText(i18n.T("Could not check for updates. Are you online?"))
				case update.Newer(current, latest):
					status.SetText(i18n.Tf("Pangolin %s is available.", latest))
					download := widget.NewButtonWithIcon(i18n.T("Download"), theme.DownloadIcon(), func() {
						u, _ := url.Parse(update.DownloadPage)
						a.fyne.OpenURL(u)
					})
					download.Importance = widget.HighImportance
					actions.Objects = []fyne.CanvasObject{download}
					actions.Refresh()
				case !update.IsRelease(current):
					status.SetText(i18n.T("This is a development build."))
				default:
					status.SetText(i18n.T("You have the latest version."))
				}
			})
		})
	})
	actions.Add(check)

	text := container.New(layout.NewCustomPaddedVBoxLayout(-theme.Padding()*3), title, status)
	return container.NewBorder(nil, nil, nil, container.NewCenter(actions), text)
}
