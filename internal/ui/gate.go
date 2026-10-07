package ui

import (
	"errors"
	"image/color"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/vault"
)

// say shows a message under the button.
func (g *gate) say(message string) {
	g.status.SetText(message)
	g.status.Show()
}

// gate is the screen shown while locked: "create" on first run, "unlock" after.
type gate struct {
	password *widget.Entry
	confirm  *widget.Entry // nil on the unlock screen
	status   *widget.Label
	button   *widget.Button
	submit   func()
}

func (a *App) showGate() {
	creating := !vault.Exists(a.dir)
	g := &gate{
		password: widget.NewPasswordEntry(),
		status:   widget.NewLabel(""),
	}
	g.password.SetPlaceHolder(i18n.T("Master password"))
	g.status.Alignment = fyne.TextAlignCenter
	g.status.Importance = widget.DangerImportance
	g.status.Hide() // shown with the first message

	title := widget.NewLabelWithStyle("Pangolin", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameHeadingText

	subtitle := muted("")
	subtitle.Alignment = fyne.TextAlignCenter

	// An invisible strut gives the card its width.
	strut := canvas.NewRectangle(color.Transparent)
	strut.SetMinSize(fyne.NewSize(360, 0))

	form := container.NewVBox(strut, container.NewCenter(logoImage(128)), title, subtitle)
	if creating {
		subtitle.SetText(i18n.T("Create your vault"))
		g.confirm = widget.NewPasswordEntry()
		g.confirm.SetPlaceHolder(i18n.T("Repeat master password"))
		g.confirm.OnSubmitted = func(string) { g.submit() }
		g.password.OnSubmitted = func(string) { a.win.Canvas().Focus(g.confirm) }
		g.button = widget.NewButton(i18n.T("Create vault"), func() { g.submit() })
		g.submit = func() { a.submitCreate(g) }

		// Explicit line breaks: a wrapping label has no usable minimum
		// height inside a centred layout.
		intro := muted(i18n.Tf("Choose a master password of at least %d characters.\nIt is never stored, so it cannot be recovered.",
			vault.MinPasswordLength))
		intro.Alignment = fyne.TextAlignCenter
		intro.SizeName = theme.SizeNameCaptionText
		form.Add(intro)
		form.Add(g.password)
		form.Add(g.confirm)
	} else {
		subtitle.SetText(i18n.T("Welcome back"))
		g.password.OnSubmitted = func(string) { g.submit() }
		g.button = widget.NewButtonWithIcon(i18n.T("Unlock"), lockIcon, func() { g.submit() })
		g.submit = func() { a.submitUnlock(g) }
		form.Add(g.password)
	}
	g.button.Importance = widget.HighImportance
	form.Add(g.button)
	form.Add(g.status)

	language := segmented(languageNames(), languageIndex(), func(i int) { a.setLanguage(i18n.Languages[i]) })

	a.gate = g
	a.win.SetContent(container.NewCenter(container.NewVBox(
		card(form),
		container.NewCenter(language),
	)))
	a.win.Canvas().Focus(g.password)
}

func (a *App) submitCreate(g *gate) {
	switch {
	case utf8.RuneCountInString(g.password.Text) < vault.MinPasswordLength:
		g.say(i18n.Tf("Use at least %d characters.", vault.MinPasswordLength))
		return
	case g.password.Text != g.confirm.Text:
		g.say(i18n.T("The two passwords do not match."))
		return
	}
	secret := []byte(g.password.Text)
	a.derive(g, i18n.T("Creating vault…"), func() (*vault.Vault, error) {
		defer crypto.Wipe(secret)
		return vault.Create(a.dir, secret, a.params)
	})
}

func (a *App) submitUnlock(g *gate) {
	if g.password.Text == "" {
		return
	}
	secret := []byte(g.password.Text)
	a.derive(g, i18n.T("Unlocking…"), func() (*vault.Vault, error) {
		defer crypto.Wipe(secret)
		return vault.Open(a.dir, secret)
	})
}

// derive runs open, which includes a slow key derivation, off the UI thread.
func (a *App) derive(g *gate, message string, open func() (*vault.Vault, error)) {
	if !a.busy.CompareAndSwap(false, true) {
		return
	}
	g.password.SetText("")
	g.password.Disable()
	if g.confirm != nil {
		g.confirm.SetText("")
		g.confirm.Disable()
	}
	g.button.Disable()
	g.status.Importance = widget.LowImportance
	g.say(message)

	go func() {
		v, err := open()
		fyne.Do(func() {
			defer a.busy.Store(false)
			if err != nil {
				g.password.Enable()
				if g.confirm != nil {
					g.confirm.Enable()
				}
				g.button.Enable()
				g.status.Importance = widget.DangerImportance
				g.say(gateMessage(err))
				a.win.Canvas().Focus(g.password)
				return
			}
			a.enter(v)
		})
	}()
}

func gateMessage(err error) string {
	switch {
	case errors.Is(err, vault.ErrUnlock):
		return i18n.T("Wrong master password, or the vault is damaged.")
	case errors.Is(err, vault.ErrWeakPassword):
		return i18n.Tf("Use at least %d characters.", vault.MinPasswordLength)
	}
	return err.Error()
}
