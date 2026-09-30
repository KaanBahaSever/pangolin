package session

import (
	"sync"
	"time"
)

// IdleTimer calls a function once no activity has been reported for a while.
type IdleTimer struct {
	mu      sync.Mutex
	timeout time.Duration
	onIdle  func()
	timer   *time.Timer
}

// NewIdleTimer returns a stopped timer. onIdle is called on its own goroutine.
func NewIdleTimer(onIdle func()) *IdleTimer {
	return &IdleTimer{onIdle: onIdle}
}

// Start arms the timer. A timeout of zero or less disables it.
func (t *IdleTimer) Start(timeout time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timeout = timeout
	t.reset()
}

// Touch reports user activity and restarts the countdown.
func (t *IdleTimer) Touch() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.timer != nil {
		t.reset()
	}
}

// Stop disarms the timer.
func (t *IdleTimer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timeout = 0
	t.reset()
}

func (t *IdleTimer) reset() {
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	if t.timeout > 0 {
		t.timer = time.AfterFunc(t.timeout, t.onIdle)
	}
}
