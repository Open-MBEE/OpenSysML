package model

import (
	"sync"
	"time"
)

// timer is the part of *time.Timer a debouncer uses, so tests can drive the
// window instead of sleeping through it.
type timer interface {
	Stop() bool
}

// Debouncer coalesces bursts of triggers per key into a single delayed call.
type Debouncer struct {
	window    time.Duration
	afterFunc func(time.Duration, func()) timer
	mu        sync.Mutex
	timers    map[string]timer
}

// NewDebouncer returns a debouncer with the given coalescing window.
func NewDebouncer(window time.Duration) *Debouncer {
	return &Debouncer{
		window:    window,
		afterFunc: func(d time.Duration, fn func()) timer { return time.AfterFunc(d, fn) },
		timers:    map[string]timer{},
	}
}

// Trigger (re)starts key's timer; fn runs once when the window elapses with no
// further trigger for that key. A trigger within the window resets the timer.
func (d *Debouncer) Trigger(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.timers[key]; ok {
		t.Stop()
	}
	// Stop cannot cancel a timer that already fired, so the callback also checks
	// that it is still the one this key is waiting on: a callback held up on the
	// lock while Trigger installs its successor is superseded, not merely late.
	var t timer
	t = d.afterFunc(d.window, func() {
		d.mu.Lock()
		if d.timers[key] != t {
			d.mu.Unlock()
			return
		}
		delete(d.timers, key)
		d.mu.Unlock()
		fn()
	})
	d.timers[key] = t
}
