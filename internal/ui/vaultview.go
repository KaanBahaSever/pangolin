package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/vault"
)

const masked = "••••••••••••"

// vaultView is the unlocked screen: search and list on the left, one entry
// or the editor on the right.
type vaultView struct {
	a       *App
	content fyne.CanvasObject

	search *widget.Entry
	list   *widget.List
	pane   *fyne.Container
	status *widget.Label

	items    []vault.Summary
	selected string // ID of the entry shown in the pane, if any
	editor   *editor
}

func newVaultView(a *App) *vaultView {
	vv := &vaultView{a: a}

	vv.search = widget.NewEntry()
	vv.search.SetPlaceHolder(i18n.T("Search"))
	vv.search.OnChanged = func(string) {
		a.touch()
		vv.reload()
		// An open editor is left alone; a shown entry is put away, which
		// also masks its password again.
		if vv.editor == nil {
			vv.showPlaceholder()
		}
	}

	vv.list = widget.NewList(
		func() int { return len(vv.items) },
		func() fyne.CanvasObject {
			title := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			title.Truncation = fyne.TextTruncateEllipsis
			user := widget.NewLabel("")
			user.Truncation = fyne.TextTruncateEllipsis
			user.Importance = widget.LowImportance
			return container.New(layout.NewCustomPaddedVBoxLayout(-theme.Padding()*2), title, user)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*fyne.Container)
			row.Objects[0].(*widget.Label).SetText(vv.items[i].Title)
			row.Objects[1].(*widget.Label).SetText(vv.items[i].Username)
		},
	)
	vv.list.OnSelected = func(i widget.ListItemID) {
		a.touch()
		vv.showDetail(vv.items[i])
	}

	add := widget.NewButtonWithIcon(i18n.T("New"), theme.ContentAddIcon(), vv.newEntry)
	add.Importance = widget.HighImportance
	settings := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() { a.showSettings() })
	lock := widget.NewButtonWithIcon(i18n.T("Lock"), theme.LogoutIcon(), a.lock)

	vv.status = widget.NewLabel("")
	vv.status.Importance = widget.LowImportance
	vv.status.Truncation = fyne.TextTruncateEllipsis

	vv.pane = container.NewStack()
	split := container.NewHSplit(vv.list, container.NewPadded(vv.pane))
	split.SetOffset(0.34)

	top := container.NewBorder(nil, nil, nil, container.NewHBox(add, settings, lock), vv.search)
	vv.content = container.NewPadded(container.NewBorder(top, vv.status, nil, nil, split))

	vv.reload()
	vv.showPlaceholder()
	return vv
}

// fail reports an error in a dialog.
func (vv *vaultView) fail(err error) {
	vv.a.showError(err)
}

// reload re-reads the list from the vault, keeping the current search.
func (vv *vaultView) reload() {
	items, err := vv.a.vault.List(vv.search.Text)
	if err != nil {
		vv.fail(err)
		return
	}
	vv.items = items
	vv.list.UnselectAll()
	vv.list.Refresh()
}

func (vv *vaultView) setPane(o fyne.CanvasObject) {
	vv.editor = nil
	vv.pane.Objects = []fyne.CanvasObject{o}
	vv.pane.Refresh()
}

func (vv *vaultView) showPlaceholder() {
	vv.selected = ""
	text := i18n.T("Select an entry")
	if len(vv.items) == 0 && vv.search.Text == "" {
		text = i18n.T("Your vault is empty.\nAdd your first entry with New.")
	}
	label := widget.NewLabel(text)
	label.Alignment = fyne.TextAlignCenter
	label.Importance = widget.LowImportance
	vv.setPane(container.NewCenter(label))
}

// show selects the entry with the given ID, clearing the search if it hides it.
func (vv *vaultView) show(id string) {
	find := func() int {
		for i, s := range vv.items {
			if s.ID == id {
				return i
			}
		}
		return -1
	}
	i := find()
	if i < 0 && vv.search.Text != "" {
		vv.search.SetText("") // triggers reload
		i = find()
	}
	if i < 0 {
		vv.showPlaceholder()
		return
	}
	vv.list.Select(i)
}

// secret decrypts a field, passes it to use and wipes it.
func (vv *vaultView) secret(id string, f vault.Field, use func(string)) {
	b, err := vv.a.vault.Secret(id, f)
	if err != nil {
		vv.fail(err)
		return
	}
	use(string(b))
	crypto.Wipe(b)
}

func (vv *vaultView) copyPassword(id string) {
	ttl := vv.a.clipboardTTL()
	vv.secret(id, vault.FieldPassword, func(s string) { vv.a.clip.CopySecret(s, ttl) })
	vv.status.SetText(i18n.Tf("Password copied. The clipboard will be cleared in %d seconds.", int(ttl.Seconds())))
}

func (vv *vaultView) copyPlain(value, message string) {
	vv.a.clip.CopyPlain(value)
	vv.status.SetText(message)
}

func (vv *vaultView) showDetail(s vault.Summary) {
	a := vv.a
	vv.selected = s.ID
	vv.status.SetText("")

	title := widget.NewLabelWithStyle(s.Title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText
	title.Truncation = fyne.TextTruncateEllipsis

	form := container.New(layout.NewFormLayout())
	addRow := func(name string, value fyne.CanvasObject, buttons ...fyne.CanvasObject) {
		label := widget.NewLabel(name)
		label.Importance = widget.LowImportance
		form.Add(label)
		form.Add(container.NewBorder(nil, nil, nil, container.NewHBox(buttons...), value))
	}
	plain := func(text string) *widget.Label {
		l := widget.NewLabel(text)
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}
	icon := func(res fyne.Resource, tapped func()) *widget.Button {
		return widget.NewButtonWithIcon("", res, func() {
			a.touch()
			tapped()
		})
	}

	if s.Username != "" {
		addRow(i18n.T("Username"), plain(s.Username),
			icon(theme.ContentCopyIcon(), func() { vv.copyPlain(s.Username, i18n.T("Username copied.")) }))
	}

	password := widget.NewLabelWithStyle(masked, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	password.Truncation = fyne.TextTruncateEllipsis
	var reveal *widget.Button
	shown := false
	reveal = icon(theme.VisibilityIcon(), func() {
		if shown {
			password.SetText(masked)
			reveal.SetIcon(theme.VisibilityIcon())
			shown = false
			return
		}
		vv.secret(s.ID, vault.FieldPassword, func(p string) {
			password.SetText(p)
			reveal.SetIcon(theme.VisibilityOffIcon())
			shown = true
		})
	})
	addRow(i18n.T("Password"), password, reveal,
		icon(theme.ContentCopyIcon(), func() { vv.copyPassword(s.ID) }))

	if s.URL != "" {
		addRow(i18n.T("URL"), plain(s.URL),
			icon(theme.ContentCopyIcon(), func() { vv.copyPlain(s.URL, i18n.T("URL copied.")) }))
	}

	if s.HasNotes {
		notes := widget.NewLabel(i18n.T("Hidden"))
		notes.Wrapping = fyne.TextWrapWord
		notes.Importance = widget.LowImportance
		var toggle *widget.Button
		notesShown := false
		toggle = icon(theme.VisibilityIcon(), func() {
			if notesShown {
				notes.SetText(i18n.T("Hidden"))
				notes.Importance = widget.LowImportance
				toggle.SetIcon(theme.VisibilityIcon())
				notesShown = false
				return
			}
			vv.secret(s.ID, vault.FieldNotes, func(n string) {
				notes.Importance = widget.MediumImportance
				notes.SetText(n)
				toggle.SetIcon(theme.VisibilityOffIcon())
				notesShown = true
			})
		})
		addRow(i18n.T("Notes"), notes, toggle)
	}

	updated := widget.NewLabel(i18n.Tf("Updated %s", s.UpdatedAt.Format("2006-01-02 15:04")))
	updated.Importance = widget.LowImportance

	edit := widget.NewButtonWithIcon(i18n.T("Edit"), theme.DocumentCreateIcon(), func() {
		a.touch()
		vv.editEntry(s)
	})
	remove := widget.NewButtonWithIcon(i18n.T("Delete"), theme.DeleteIcon(), func() {
		a.touch()
		a.showConfirm(i18n.T("Delete entry"), i18n.Tf("Delete “%s”? This cannot be undone.", s.Title), i18n.T("Delete"), func(ok bool) {
			if !ok {
				return
			}
			if err := a.vault.Delete(s.ID); err != nil {
				vv.fail(err)
				return
			}
			vv.reload()
			vv.showPlaceholder()
			vv.status.SetText(i18n.T("Entry deleted."))
		})
	})
	remove.Importance = widget.DangerImportance

	footer := container.NewBorder(nil, nil, updated, container.NewHBox(edit, remove))
	vv.setPane(container.NewBorder(title, footer, nil, nil, container.NewVScroll(form)))
}
