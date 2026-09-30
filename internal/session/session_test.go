package session

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeClipboard struct {
	mu sync.Mutex
	s  string
}

func (c *fakeClipboard) Content() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.s
}

func (c *fakeClipboard) SetContent(s string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.s = s
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestClipboardClearedAfterTTL(t *testing.T) {
	cb := &fakeClipboard{}
	g := NewClipboardGuard(cb, nil)
	g.CopySecret("hunter2", 30*time.Millisecond)
	if cb.Content() != "hunter2" {
		t.Fatal("secret was not copied")
	}
	eventually(t, "clipboard to clear", func() bool { return cb.Content() == "" })
}

func TestClipboardLeftAloneIfUserCopiedSomethingElse(t *testing.T) {
	cb := &fakeClipboard{}
	g := NewClipboardGuard(cb, nil)
	g.CopySecret("hunter2", 30*time.Millisecond)
	cb.SetContent("the user's own text")
	time.Sleep(120 * time.Millisecond)
	if cb.Content() != "the user's own text" {
		t.Fatalf("clipboard = %q", cb.Content())
	}
}

func TestClipboardClearNow(t *testing.T) {
	cb := &fakeClipboard{}
	g := NewClipboardGuard(cb, nil)
	g.CopySecret("hunter2", time.Hour)
	g.Clear()
	if cb.Content() != "" {
		t.Fatal("Clear did not clear")
	}
	cb.SetContent("later")
	g.Clear()
	if cb.Content() != "later" {
		t.Fatal("Clear removed content the guard does not own")
	}
}

func TestClipboardPlainCopyIsNotCleared(t *testing.T) {
	cb := &fakeClipboard{}
	g := NewClipboardGuard(cb, nil)
	g.CopySecret("hunter2", 30*time.Millisecond)
	g.CopyPlain("octocat")
	time.Sleep(120 * time.Millisecond)
	g.Clear()
	if cb.Content() != "octocat" {
		t.Fatalf("clipboard = %q", cb.Content())
	}
}

func TestClipboardNewSecretRestartsTimer(t *testing.T) {
	cb := &fakeClipboard{}
	g := NewClipboardGuard(cb, nil)
	g.CopySecret("first", 60*time.Millisecond)
	time.Sleep(40 * time.Millisecond)
	g.CopySecret("second", 200*time.Millisecond)
	time.Sleep(80 * time.Millisecond)
	if cb.Content() != "second" {
		t.Fatal("the first secret's timer cleared the second secret")
	}
	eventually(t, "clipboard to clear", func() bool { return cb.Content() == "" })
}

func TestIdleTimerFires(t *testing.T) {
	var fired atomic.Int32
	it := NewIdleTimer(func() { fired.Add(1) })
	it.Start(30 * time.Millisecond)
	eventually(t, "idle timer", func() bool { return fired.Load() == 1 })
}

func TestIdleTimerTouchPostpones(t *testing.T) {
	var fired atomic.Int32
	it := NewIdleTimer(func() { fired.Add(1) })
	it.Start(100 * time.Millisecond)
	for i := 0; i < 6; i++ {
		time.Sleep(40 * time.Millisecond)
		it.Touch()
	}
	if fired.Load() != 0 {
		t.Fatal("fired despite activity")
	}
	eventually(t, "idle timer", func() bool { return fired.Load() == 1 })
}

func TestIdleTimerStopAndDisabled(t *testing.T) {
	var fired atomic.Int32
	it := NewIdleTimer(func() { fired.Add(1) })
	it.Start(30 * time.Millisecond)
	it.Stop()
	it.Touch() // must not re-arm a stopped timer
	it2 := NewIdleTimer(func() { fired.Add(1) })
	it2.Start(0)
	it2.Touch()
	time.Sleep(100 * time.Millisecond)
	if fired.Load() != 0 {
		t.Fatal("a stopped or disabled timer fired")
	}
}
