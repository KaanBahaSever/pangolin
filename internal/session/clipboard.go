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
	// gen counts copies. A timer only clears the copy it was started for:
	// one that already fired and is waiting to run must not cut short the
	// delay of a newer secret.
	gen uint64
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
	g.gen++
	if ttl > 0 {
		gen := g.gen
		g.timer = time.AfterFunc(ttl, func() { g.run(func() { g.expire(gen) }) })
	}
}

// expire clears the clipboard for the copy numbered gen, if no newer copy
// has been made since.
func (g *ClipboardGuard) expire(gen uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.gen == gen {
		g.clear()
	}
}

// CopyPlain copies a non-secret value. Nothing is scheduled.
func (g *ClipboardGuard) CopyPlain(value string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopTimer()
	g.owned = false
	g.gen++
	g.cb.SetContent(value)
}

// Clear empties the clipboard if it still holds the last secret copied through
// the guard. Anything the user copied since is left alone.
func (g *ClipboardGuard) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.clear()
}

// clear is Clear for callers that hold g.mu.
func (g *ClipboardGuard) clear() {
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
