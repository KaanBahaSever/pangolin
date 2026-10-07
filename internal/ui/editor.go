package ui

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
	"github.com/KaanBahaSever/pangolin/internal/generator"
	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/vault"
)

// editor is the form for a new or existing entry.
type editor struct {
	id       string // empty for a new entry
	title    *widget.Entry
	username *widget.Entry
	url      *widget.Entry
	password *widget.Entry
	notes    *widget.Entry
	save     func()
}

func (vv *vaultView) newEntry() {
	vv.a.touch()
	vv.list.UnselectAll()
	vv.showEditor(&editor{}, i18n.T("New entry"))
}

func (vv *vaultView) editEntry(s vault.Summary) {
	e := &editor{id: s.ID}
	vv.showEditor(e, i18n.T("Edit entry"))
	e.title.SetText(s.Title)
	e.username.SetText(s.Username)
	e.url.SetText(s.URL)
	vv.secret(s.ID, vault.FieldPassword, e.password.SetText)
	vv.secret(s.ID, vault.FieldNotes, e.notes.SetText)
}

// labelled puts a caption above a field.
func labelled(name string, field fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedVBoxLayout(0), caption(name), field)
}

func (vv *vaultView) showEditor(e *editor, title string) {
	a := vv.a
	touch := func(string) { a.touch() }

	e.title = widget.NewEntry()
	e.title.SetPlaceHolder(i18n.T("e.g. GitHub"))
	e.username = widget.NewEntry()
	e.username.SetPlaceHolder(i18n.T("e.g. name@example.com"))
	e.url = widget.NewEntry()
	e.url.SetPlaceHolder(i18n.T("e.g. github.com"))
	e.password = widget.NewPasswordEntry()
	e.notes = widget.NewMultiLineEntry()
	e.notes.Wrapping = fyne.TextWrapWord
	e.notes.SetMinRowsVisible(3)
	e.notes.SetPlaceHolder(i18n.T("Recovery codes, security questions…"))
	for _, entry := range []*widget.Entry{e.title, e.username, e.url, e.password, e.notes} {
		entry.OnChanged = touch
	}

	generate := widget.NewButtonWithIcon(i18n.T("Generate"), theme.ViewRefreshIcon(), func() {
		a.touch()
		a.showGenerator(e.password)
	})

	e.save = func() {
		a.touch()
		entry := vault.Entry{
			ID:       e.id,
			Title:    e.title.Text,
			Username: e.username.Text,
			URL:      e.url.Text,
			Password: []byte(e.password.Text),
			Notes:    []byte(e.notes.Text),
		}
		defer crypto.Wipe(entry.Password)
		defer crypto.Wipe(entry.Notes)

		id, err := e.id, error(nil)
		if id == "" {
			id, err = a.vault.Add(entry)
		} else {
			err = a.vault.Update(entry)
		}
		if errors.Is(err, vault.ErrInvalidEntry) {
			vv.say(i18n.T("An entry needs a title."))
			a.win.Canvas().Focus(e.title)
			return
		}
		if err != nil {
			vv.fail(err)
			return
		}
		e.password.SetText("")
		e.notes.SetText("")
		vv.reload()
		vv.show(id)
		vv.say(i18n.T("Saved."))
	}
	cancel := func() {
		a.touch()
		e.password.SetText("")
		e.notes.SetText("")
		if e.id != "" {
			vv.show(e.id)
			return
		}
		vv.showPlaceholder()
	}

	saveButton := widget.NewButtonWithIcon(i18n.T("Save"), theme.ConfirmIcon(), e.save)
	saveButton.Importance = widget.HighImportance
	cancelButton := widget.NewButton(i18n.T("Cancel"), cancel)
	cancelButton.Importance = widget.LowImportance

	form := card(
		labelled(i18n.T("Title"), e.title),
		labelled(i18n.T("Username"), e.username),
		labelled(i18n.T("Password"), container.NewBorder(nil, nil, nil, generate, e.password)),
		labelled(i18n.T("Website"), e.url),
		labelled(i18n.T("Notes"), e.notes),
	)
	buttons := container.NewHBox(layout.NewSpacer(), cancelButton, saveButton)

	vv.setPage(pageEditor, scrollPage(heading(title), form, buttons))
	vv.editor = e
	a.win.Canvas().Focus(e.title)
}

// showGenerator offers a freshly generated password, with its length and
// character classes adjustable, and writes it into target on "Use".
func (a *App) showGenerator(target *widget.Entry) {
	opts := generator.DefaultOptions()

	preview := widget.NewLabelWithStyle("", fyne.TextAlignCenter, fyne.TextStyle{Monospace: true})
	preview.Wrapping = fyne.TextWrapBreak
	current := ""
	regenerate := func() {
		p, err := generator.Generate(opts)
		if err != nil {
			current = ""
			preview.SetText(i18n.T("Select at least one kind of character."))
			return
		}
		current = p
		preview.SetText(p)
	}

	length := caption("")
	slider := widget.NewSlider(generator.MinLength, 64)
	slider.Step = 1
	slider.SetValue(float64(opts.Length))
	slider.OnChanged = func(v float64) {
		opts.Length = int(v)
		length.SetText(i18n.Tf("%d characters", opts.Length))
		regenerate()
	}
	length.SetText(i18n.Tf("%d characters", opts.Length))

	check := func(label string, field *bool) *widget.Check {
		c := widget.NewCheck(label, nil)
		c.SetChecked(*field)
		c.OnChanged = func(on bool) {
			*field = on
			regenerate()
		}
		return c
	}
	regenerate()

	content := container.NewVBox(
		card(container.NewBorder(nil, nil, nil, flatButton(theme.ViewRefreshIcon(), regenerate), preview)),
		length, slider,
		container.NewGridWithColumns(2,
			check("a–z", &opts.Lower), check("A–Z", &opts.Upper),
			check("0–9", &opts.Digits), check(i18n.T("Symbols"), &opts.Symbols)),
	)
	d := dialog.NewCustomConfirm(i18n.T("Generate password"), i18n.T("Use"), i18n.T("Cancel"), content, func(ok bool) {
		a.touch()
		if ok && current != "" {
			target.SetText(current)
		}
	}, a.win)
	d.Resize(fyne.NewSize(420, 0))
	d.Show()
}
