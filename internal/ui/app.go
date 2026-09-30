// Package ui is the Fyne front end. It holds no keys: everything secret is
// requested from the vault at the moment it is needed.
package ui

import (
	"runtime"
	"runtime/debug"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/KaanBahaSever/pangolin/assets"
	"github.com/KaanBahaSever/pangolin/internal/crypto"
	"github.com/KaanBahaSever/pangolin/internal/i18n"
	"github.com/KaanBahaSever/pangolin/internal/session"
	"github.com/KaanBahaSever/pangolin/internal/vault"
)

const (
	appID = "io.github.kaanbahasever.pangolin"

	prefAutoLockMinutes  = "autolock_minutes"
	prefClipboardSeconds = "clipboard_seconds"
	prefLanguage         = "language"

	defaultAutoLockMinutes  = 5
	defaultClipboardSeconds = 20
)

var logo = fyne.NewStaticResource("logo.svg", assets.LogoSVG)

// App is the application window and the session behind it.
type App struct {
	fyne   fyne.App
	win    fyne.Window
	dir    string
	params crypto.KDFParams

	vault *vault.Vault
	clip  *session.ClipboardGuard
	idle  *session.IdleTimer

	gate *gate      // the create or unlock screen, while locked
	view *vaultView // the vault screen, while unlocked

	// busy is set while a key derivation runs in the background.
	busy atomic.Bool
}

// Run starts Pangolin with the vault in dir and blocks until the window closes.
func Run(dir string) {
	a := New(app.NewWithID(appID), dir, crypto.DefaultKDFParams())
	a.win.ShowAndRun()
	// Normally the close handler has locked already; this covers other exits.
	if a.vault != nil {
		a.vault.Close()
	}
}

// New builds the application on an existing Fyne app. params is used only
// when a new vault is created.
func New(fa fyne.App, dir string, params crypto.KDFParams) *App {
	a := &App{fyne: fa, dir: dir, params: params}
	fa.Settings().SetTheme(newTheme())
	fa.SetIcon(logo)

	a.win = fa.NewWindow("Pangolin")
	a.win.Resize(fyne.NewSize(900, 580))
	a.win.SetMaster()
	// Lock while the event loop still runs, so the clipboard can be cleared.
	a.win.SetCloseIntercept(func() {
		a.lock()
		a.win.Close()
	})

	// The saved choice wins; on first run the system language decides.
	i18n.Set(i18n.Lang(fa.Preferences().StringWithFallback(prefLanguage,
		string(i18n.FromLocale(string(fyne.CurrentDevice().Locale()))))))

	a.clip = session.NewClipboardGuard(fa.Clipboard(), fyne.Do)
	a.idle = session.NewIdleTimer(func() { fyne.Do(a.lock) })

	a.showGate()
	return a
}

func (a *App) autoLock() time.Duration {
	return time.Duration(a.fyne.Preferences().IntWithFallback(prefAutoLockMinutes, defaultAutoLockMinutes)) * time.Minute
}

func (a *App) clipboardTTL() time.Duration {
	return time.Duration(a.fyne.Preferences().IntWithFallback(prefClipboardSeconds, defaultClipboardSeconds)) * time.Second
}

// setLanguage stores the language and redraws the current screen in it.
func (a *App) setLanguage(l i18n.Lang) {
	a.touch()
	i18n.Set(l)
	a.fyne.Preferences().SetString(prefLanguage, string(l))
	a.closeDialogs()
	if a.vault == nil {
		a.showGate()
		return
	}
	a.view = newVaultView(a)
	a.win.SetContent(a.view.content)
}

// closeDialogs removes everything layered above the window content.
func (a *App) closeDialogs() {
	overlays := a.win.Canvas().Overlays()
	for top := overlays.Top(); top != nil; top = overlays.Top() {
		overlays.Remove(top)
	}
}

// touch reports user activity to the idle timer.
func (a *App) touch() {
	a.idle.Touch()
}

// enter switches to the vault screen after a successful create or unlock.
func (a *App) enter(v *vault.Vault) {
	a.vault = v
	a.gate = nil
	a.idle.Start(a.autoLock())
	a.view = newVaultView(a)
	a.win.SetContent(a.view.content)
	a.win.Canvas().Focus(a.view.search)
}

// lock destroys the keys, clears what Pangolin left on the clipboard and
// returns to the unlock screen. It does nothing if the vault is already locked.
func (a *App) lock() {
	if a.vault == nil {
		return
	}
	a.idle.Stop()
	a.clip.Clear()
	a.vault.Close()
	a.vault = nil
	a.view = nil

	// Dialogs sit above the content and would survive SetContent.
	a.closeDialogs()
	a.showGate()

	// Widget text cannot be wiped. Dropping the old screen and collecting it
	// at least returns that memory for reuse. See architecture.md §7.4.
	runtime.GC()
	debug.FreeOSMemory()
}
