package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/KaanBahaSever/pangolin/internal/crypto"
	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/vault"
)

const masked = "••••••••••••"

// page is what the content area on the right shows.
type page int

const (
	pageEmpty page = iota
	pageDetail
	pageEditor
	pageSettings
)

// vaultView is the unlocked screen: a sidebar with search and the list of
// entries, and a content area for one entry, the editor or the settings.
type vaultView struct {
	a       *App
	content fyne.CanvasObject

	search *widget.Entry
	list   *widget.List
	pane   *fyne.Container
	status *widget.Label

	items    []vault.Summary
	page     page
	selected string // ID of the entry shown, if any
	editor   *editor

	statusTimer *time.Timer
}

func newVaultView(a *App) *vaultView {
	vv := &vaultView{a: a}

	vv.search = widget.NewEntry()
	vv.search.SetPlaceHolder(i18n.T("Search"))
	vv.search.ActionItem = widget.NewIcon(theme.SearchIcon())
	vv.search.OnChanged = func(string) {
		a.touch()
		vv.reload()
		// An open editor or the settings stay; a shown entry is put away,
		// which also masks its password again.
		if vv.page == pageDetail || vv.page == pageEmpty {
			vv.showPlaceholder()
		}
	}

	vv.list = widget.NewList(
		func() int { return len(vv.items) },
		func() fyne.CanvasObject {
			title := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			title.Truncation = fyne.TextTruncateEllipsis
			user := muted("")
			user.Truncation = fyne.TextTruncateEllipsis
			user.SizeName = theme.SizeNameCaptionText
			text := container.New(layout.NewCustomPaddedVBoxLayout(-theme.Padding()*3), title, user)
			// The avatar slot has a fixed size, so the row's layout never
			// depends on which avatar is in it.
			slot := canvas.NewRectangle(color.Transparent)
			slot.SetMinSize(fyne.NewSize(34, 34))
			return container.NewBorder(nil, nil, container.NewStack(slot), nil, text)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			s := vv.items[i]
			row := o.(*fyne.Container)
			text := row.Objects[0].(*fyne.Container)
			text.Objects[0].(*widget.Label).SetText(s.Title)
			user := text.Objects[1].(*widget.Label)
			user.SetText(s.Username)
			// The avatar depends on the title, so it is rebuilt for each row.
			slot := row.Objects[1].(*fyne.Container)
			slot.Objects = []fyne.CanvasObject{slot.Objects[0], avatar(s.Title, 34)}
			slot.Refresh()
		},
	)
	vv.list.OnSelected = func(i widget.ListItemID) {
		a.touch()
		vv.showDetail(vv.items[i])
	}

	name := widget.NewLabelWithStyle("Pangolin", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	settings := flatButton(theme.SettingsIcon(), vv.showSettings)
	header := container.NewBorder(nil, nil, container.NewHBox(logoImage(38), name), settings)

	add := widget.NewButtonWithIcon(i18n.T("New entry"), theme.ContentAddIcon(), vv.newEntry)
	add.Importance = widget.HighImportance
	lock := widget.NewButtonWithIcon(i18n.T("Lock"), lockIcon, a.lock)
	lock.Importance = widget.LowImportance
	footer := container.NewBorder(nil, nil, nil, lock, add)

	// A strut fixes the sidebar's width.
	strut := canvas.NewRectangle(color.Transparent)
	strut.SetMinSize(fyne.NewSize(290, 0))
	sidebar := container.NewStack(
		newSurface(colorNameSidebar, "", 0),
		strut,
		container.New(layout.NewCustomPaddedLayout(12, 12, 12, 12),
			container.NewBorder(container.NewVBox(header, vv.search), footer, nil, nil, vv.list)),
	)

	vv.status = muted("")
	vv.status.Truncation = fyne.TextTruncateEllipsis
	vv.status.SizeName = theme.SizeNameCaptionText

	vv.pane = container.NewStack()
	main := container.New(layout.NewCustomPaddedLayout(18, 8, 26, 12),
		container.NewBorder(nil, vv.status, nil, nil, vv.pane))

	vv.content = container.NewBorder(nil, nil, container.NewHBox(sidebar, widget.NewSeparator()), nil, main)

	vv.reload()
	vv.showPlaceholder()
	return vv
}

// fail reports an error in a dialog.
func (vv *vaultView) fail(err error) {
	vv.a.showError(err)
}

// say shows a short message under the content for a few seconds.
func (vv *vaultView) say(message string) {
	vv.status.SetText(message)
	if vv.statusTimer != nil {
		vv.statusTimer.Stop()
	}
	vv.statusTimer = time.AfterFunc(6*time.Second, func() {
		fyne.Do(func() {
			if vv.status.Text == message {
				vv.status.SetText("")
			}
		})
	})
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

// setPage replaces the content area. Everything is top-aligned and scrolls,
// so a short page never leaves a gap in the middle.
func (vv *vaultView) setPage(p page, o fyne.CanvasObject) {
	vv.page = p
	if p != pageEditor {
		vv.editor = nil
	}
	if p != pageDetail {
		vv.selected = ""
	}
	vv.pane.Objects = []fyne.CanvasObject{o}
	// Pages scroll; the right margin keeps card edges clear of the scroll bar.
	vv.pane.Refresh()
}

func (vv *vaultView) showPlaceholder() {
	title, hint := i18n.T("Nothing selected"), i18n.T("Pick an entry on the left, or add a new one.")
	if len(vv.items) == 0 && vv.search.Text == "" {
		title, hint = i18n.T("Your vault is empty"), i18n.T("Add your first entry with New entry.")
	}
	t := widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	t.SizeName = theme.SizeNameSubHeadingText
	h := muted(hint)
	h.Alignment = fyne.TextAlignCenter
	vv.list.UnselectAll()
	vv.setPage(pageEmpty, container.NewCenter(container.NewVBox(container.NewCenter(logoImage(120)), t, h)))
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
	vv.say(i18n.Tf("Password copied. The clipboard will be cleared in %d seconds.", int(ttl.Seconds())))
}

func (vv *vaultView) copyPlain(value, message string) {
	vv.a.clip.CopyPlain(value)
	vv.say(message)
}

func (vv *vaultView) showDetail(s vault.Summary) {
	a := vv.a
	action := func(icon fyne.Resource, tapped func()) *widget.Button {
		return flatButton(icon, func() {
			a.touch()
			tapped()
		})
	}
	field := func(name string, value fyne.CanvasObject, actions ...fyne.CanvasObject) fyne.CanvasObject {
		label := container.New(layout.NewCustomPaddedVBoxLayout(-theme.Padding()*2), caption(name), value)
		return container.NewBorder(nil, nil, nil, container.NewCenter(container.NewHBox(actions...)), label)
	}
	value := func(text string) *widget.Label {
		l := widget.NewLabel(text)
		l.Truncation = fyne.TextTruncateEllipsis
		return l
	}

	var rows []fyne.CanvasObject
	if s.Username != "" {
		rows = append(rows, field(i18n.T("Username"), value(s.Username),
			action(theme.ContentCopyIcon(), func() { vv.copyPlain(s.Username, i18n.T("Username copied.")) })))
	}

	password := widget.NewLabelWithStyle(masked, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	password.Truncation = fyne.TextTruncateEllipsis
	var reveal *widget.Button
	shown := false
	reveal = action(theme.VisibilityIcon(), func() {
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
	rows = append(rows, field(i18n.T("Password"), password, reveal,
		action(theme.ContentCopyIcon(), func() { vv.copyPassword(s.ID) })))

	if s.URL != "" {
		rows = append(rows, field(i18n.T("Website"), value(s.URL),
			action(theme.ContentCopyIcon(), func() { vv.copyPlain(s.URL, i18n.T("Website copied.")) })))
	}

	if s.HasNotes {
		notes := muted(i18n.T("Hidden"))
		notes.Wrapping = fyne.TextWrapWord
		var toggle *widget.Button
		notesShown := false
		toggle = action(theme.VisibilityIcon(), func() {
			if notesShown {
				notes.Importance = widget.LowImportance
				notes.SetText(i18n.T("Hidden"))
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
		rows = append(rows, field(i18n.T("Notes"), notes, toggle))
	}

	edit := widget.NewButtonWithIcon(i18n.T("Edit"), theme.DocumentCreateIcon(), func() {
		a.touch()
		vv.editEntry(s)
	})
	remove := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
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
			vv.say(i18n.T("Entry deleted."))
		})
	})
	remove.Importance = widget.DangerImportance

	subtitle := muted(s.URL)
	if s.URL == "" {
		subtitle.SetText(s.Username)
	}
	subtitle.Truncation = fyne.TextTruncateEllipsis
	titles := container.New(layout.NewCustomPaddedVBoxLayout(-theme.Padding()*2), heading(s.Title), subtitle)
	header := container.NewBorder(nil, nil, avatar(s.Title, 52),
		container.NewCenter(container.NewHBox(edit, remove)), titles)

	updated := muted(i18n.Tf("Updated %s", s.UpdatedAt.Format("2006-01-02 15:04")))
	updated.SizeName = theme.SizeNameCaptionText

	vv.setPage(pageDetail, scrollPage(header, cardRows(rows...), updated))
	vv.selected = s.ID
}
