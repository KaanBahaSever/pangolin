package ui

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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
	vv.selected = ""
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

func (vv *vaultView) showEditor(e *editor, heading string) {
	a := vv.a
	vv.status.SetText("")
	touch := func(string) { a.touch() }

	e.title = widget.NewEntry()
	e.title.SetPlaceHolder(i18n.T("e.g. GitHub"))
	e.username = widget.NewEntry()
	e.url = widget.NewEntry()
	e.password = widget.NewPasswordEntry()
	e.notes = widget.NewMultiLineEntry()
	e.notes.Wrapping = fyne.TextWrapWord
	e.notes.SetMinRowsVisible(4)
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
			vv.status.SetText(i18n.T("An entry needs a title."))
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
		vv.status.SetText(i18n.T("Saved."))
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

	form := widget.NewForm(
		widget.NewFormItem(i18n.T("Title"), e.title),
		widget.NewFormItem(i18n.T("Username"), e.username),
		widget.NewFormItem(i18n.T("Password"), container.NewBorder(nil, nil, nil, generate, e.password)),
		widget.NewFormItem(i18n.T("URL"), e.url),
		widget.NewFormItem(i18n.T("Notes"), e.notes),
	)
	saveButton := widget.NewButtonWithIcon(i18n.T("Save"), theme.DocumentSaveIcon(), e.save)
	saveButton.Importance = widget.HighImportance
	buttons := container.NewHBox(widget.NewButton(i18n.T("Cancel"), cancel), saveButton)

	title := widget.NewLabelWithStyle(heading, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText

	vv.setPane(container.NewBorder(title, container.NewBorder(nil, nil, nil, buttons), nil, nil,
		container.NewVScroll(form)))
	vv.editor = e
	a.win.Canvas().Focus(e.title)
}

// showGenerator asks for length and character classes and writes a new
// password into target.
func (a *App) showGenerator(target *widget.Entry) {
	opts := generator.DefaultOptions()

	length := widget.NewLabel("")
	slider := widget.NewSlider(generator.MinLength, 64)
	slider.Step = 1
	slider.OnChanged = func(v float64) {
		opts.Length = int(v)
		length.SetText(i18n.Tf("%d characters", opts.Length))
	}
	slider.SetValue(float64(opts.Length))
	slider.OnChanged(slider.Value)

	check := func(label string, field *bool) *widget.Check {
		c := widget.NewCheck(label, func(on bool) { *field = on })
		c.SetChecked(*field)
		return c
	}
	content := container.NewVBox(
		length, slider,
		container.NewGridWithColumns(2,
			check("a–z", &opts.Lower), check("A–Z", &opts.Upper),
			check("0–9", &opts.Digits), check(i18n.T("Symbols"), &opts.Symbols)),
	)
	dialog.ShowCustomConfirm(i18n.T("Generate password"), i18n.T("Use"), i18n.T("Cancel"), content, func(ok bool) {
		a.touch()
		if !ok {
			return
		}
		password, err := generator.Generate(opts)
		if err != nil {
			a.showInfo(i18n.T("Generate password"), i18n.T("Select at least one kind of character."))
			return
		}
		target.SetText(password)
	}, a.win)
}
