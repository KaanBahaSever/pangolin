package ui

import (
	"errors"
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

	image := canvas.NewImageFromResource(logo)
	image.FillMode = canvas.ImageFillContain
	image.SetMinSize(fyne.NewSize(120, 120))

	title := widget.NewLabelWithStyle("Pangolin", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameHeadingText

	// An invisible strut gives the column its width.
	strut := canvas.NewRectangle(nil)
	strut.SetMinSize(fyne.NewSize(360, 0))

	column := container.NewVBox(image, title, strut)
	if creating {
		g.confirm = widget.NewPasswordEntry()
		g.confirm.SetPlaceHolder(i18n.T("Repeat master password"))
		g.confirm.OnSubmitted = func(string) { g.submit() }
		g.password.OnSubmitted = func(string) { a.win.Canvas().Focus(g.confirm) }
		g.button = widget.NewButton(i18n.T("Create vault"), func() { g.submit() })
		g.submit = func() { a.submitCreate(g) }

		// Explicit line breaks: a wrapping label has no usable minimum
		// height inside a centred layout.
		intro := widget.NewLabel(i18n.Tf(
			"Choose a master password of at least %d characters.\nIt is never stored and cannot be recovered.\nIf you forget it, the vault is lost.",
			vault.MinPasswordLength))
		intro.Alignment = fyne.TextAlignCenter
		intro.Importance = widget.LowImportance
		column.Add(intro)
		column.Add(g.password)
		column.Add(g.confirm)
	} else {
		g.password.OnSubmitted = func(string) { g.submit() }
		g.button = widget.NewButton(i18n.T("Unlock"), func() { g.submit() })
		g.submit = func() { a.submitUnlock(g) }
		column.Add(g.password)
	}
	g.button.Importance = widget.HighImportance
	column.Add(g.button)
	column.Add(g.status)
	column.Add(container.NewCenter(a.languageSelect()))

	a.gate = g
	a.win.SetContent(container.NewCenter(column))
	a.win.Canvas().Focus(g.password)
}

func (a *App) submitCreate(g *gate) {
	switch {
	case utf8.RuneCountInString(g.password.Text) < vault.MinPasswordLength:
		g.status.SetText(i18n.Tf("Use at least %d characters.", vault.MinPasswordLength))
		return
	case g.password.Text != g.confirm.Text:
		g.status.SetText(i18n.T("The two passwords do not match."))
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
	g.status.SetText(message)

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
				g.status.SetText(gateMessage(err))
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
