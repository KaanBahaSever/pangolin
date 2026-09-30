package ui

import (
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/KaanBahaSever/pangolin/internal/i18n"
)

// These wrap Fyne's dialogs so that every button follows the language chosen
// in Pangolin rather than the system locale.

func (a *App) showInfo(title, message string) {
	dialog.NewCustom(title, i18n.T("OK"), widget.NewLabel(message), a.win).Show()
}

func (a *App) showError(err error) {
	a.showInfo(i18n.T("Error"), err.Error())
}

func (a *App) showConfirm(title, message, confirm string, done func(bool)) {
	dialog.NewCustomConfirm(title, confirm, i18n.T("Cancel"), widget.NewLabel(message), done, a.win).Show()
}
