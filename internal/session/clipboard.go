// Package session holds the parts of an unlocked session that are about time:
// clearing the clipboard and locking after inactivity.
package session

import (
	"crypto/sha256"
	"crypto/subtle"
	"sync"
	"time"
)

// Clipboard is the part of the system clipboard the guard needs.
type Clipboard interface {
	Content() string
	SetContent(string)
}

// ClipboardGuard puts secrets on the clipboard and takes them off again.
type ClipboardGuard struct {
	cb Clipboard
	// run executes a function on the thread that may touch the clipboard.
	run func(func())

	mu    sync.Mutex
	owned bool
	sum   [sha256.Size]byte
	timer *time.Timer
}

// NewClipboardGuard returns a guard for cb. run is used for clipboard access
// from the timer; pass nil to call it directly.
func NewClipboardGuard(cb Clipboard, run func(func())) *ClipboardGuard {
	if run == nil {
		run = func(f func()) { f() }
	}
	return &ClipboardGuard{cb: cb, run: run}
}

// CopySecret copies value and schedules its removal after ttl. A ttl of zero
// or less disables the timer; the value is still cleared by Clear.
func (g *ClipboardGuard) CopySecret(value string, ttl time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopTimer()
	g.cb.SetContent(value)
	// Only a hash is kept, to recognise the value later without storing it.
	g.sum = sha256.Sum256([]byte(value))
	g.owned = true
	if ttl > 0 {
		g.timer = time.AfterFunc(ttl, func() { g.run(g.Clear) })
	}
}

// CopyPlain copies a non-secret value. Nothing is scheduled.
func (g *ClipboardGuard) CopyPlain(value string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopTimer()
	g.owned = false
	g.cb.SetContent(value)
}

// Clear empties the clipboard if it still holds the last secret copied through
// the guard. Anything the user copied since is left alone.
func (g *ClipboardGuard) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopTimer()
	if !g.owned {
		return
	}
	g.owned = false
	current := sha256.Sum256([]byte(g.cb.Content()))
	if subtle.ConstantTimeCompare(current[:], g.sum[:]) == 1 {
		g.cb.SetContent("")
	}
	g.sum = [sha256.Size]byte{}
}

func (g *ClipboardGuard) stopTimer() {
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
}
