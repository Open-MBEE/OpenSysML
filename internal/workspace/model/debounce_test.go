package model

import (
	"testing"
	"time"
)

// fakeClock hands out timers whose window only elapses when the test says so,
// so a burst coalesces (or fails to) for the reason under test rather than
// because a loaded machine stretched a sleep past the window.
type fakeClock struct {
	now     time.Duration
	pending []*fakeTimer
}

type fakeTimer struct {
	due   time.Duration
	fn    func()
	fired bool
	dead  bool
}

func (t *fakeTimer) Stop() bool {
	if t.fired || t.dead {
		return false
	}
	t.dead = true
	return true
}

func (c *fakeClock) afterFunc(d time.Duration, fn func()) timer {
	t := &fakeTimer{due: c.now + d, fn: fn}
	c.pending = append(c.pending, t)
	return t
}

// advance moves the clock and runs every callback that comes due, as the
// runtime would.
func (c *fakeClock) advance(d time.Duration) {
	c.now += d
	for _, t := range c.pending {
		if !t.fired && !t.dead && t.due <= c.now {
			t.fired = true
			t.fn()
		}
	}
}

func newFakeDebouncer(window time.Duration) (*Debouncer, *fakeClock) {
	c := &fakeClock{}
	d := NewDebouncer(window)
	d.afterFunc = c.afterFunc
	return d, c
}

func TestDebouncerCoalescesBurst(t *testing.T) {
	d, clock := newFakeDebouncer(30 * time.Millisecond)
	var calls int
	for i := 0; i < 5; i++ {
		d.Trigger("a.sysml", func() { calls++ })
		clock.advance(3 * time.Millisecond)
	}
	if calls != 0 {
		t.Fatalf("calls = %d during the burst, want 0 (window has not elapsed)", calls)
	}
	clock.advance(30 * time.Millisecond)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (burst coalesced)", calls)
	}
	clock.advance(time.Second)
	if calls != 1 {
		t.Fatalf("calls = %d after the window, want 1 (no stale timer left)", calls)
	}
}

func TestDebouncerTriggerAfterWindowRunsAgain(t *testing.T) {
	d, clock := newFakeDebouncer(30 * time.Millisecond)
	var calls int
	d.Trigger("a.sysml", func() { calls++ })
	clock.advance(30 * time.Millisecond)
	d.Trigger("a.sysml", func() { calls++ })
	clock.advance(30 * time.Millisecond)
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 (separate windows are separate calls)", calls)
	}
}

func TestDebouncerDistinctKeysIndependent(t *testing.T) {
	d, clock := newFakeDebouncer(20 * time.Millisecond)
	var a, b int
	d.Trigger("a.sysml", func() { a++ })
	d.Trigger("b.sysml", func() { b++ })
	clock.advance(20 * time.Millisecond)
	if a != 1 || b != 1 {
		t.Fatalf("a=%d b=%d, want 1 and 1", a, b)
	}
}

// A timer that fires just as the next trigger arrives cannot be stopped: its
// callback is already running and only waiting for the lock. It must not run
// the work its successor now owns, and must not drop the successor's entry.
func TestDebouncerSupersededCallbackDoesNotRun(t *testing.T) {
	d, clock := newFakeDebouncer(30 * time.Millisecond)
	var calls int

	d.Trigger("a.sysml", func() { calls++ })
	stale := clock.pending[0]
	stale.fired = true // fired, so the next trigger's Stop cannot take it back

	d.Trigger("a.sysml", func() { calls++ })
	stale.fn() // the callback finally gets the lock, after its successor is in

	if calls != 0 {
		t.Fatalf("calls = %d, want 0 (superseded callback must not run)", calls)
	}
	clock.advance(60 * time.Millisecond)
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (the successor still owes one call)", calls)
	}
}

// The debouncer really is wired to the runtime clock, not only to the fake one.
func TestDebouncerRunsOnRealClock(t *testing.T) {
	d := NewDebouncer(10 * time.Millisecond)
	done := make(chan struct{})
	d.Trigger("a.sysml", func() { close(done) })
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("debounced call never ran")
	}
}
