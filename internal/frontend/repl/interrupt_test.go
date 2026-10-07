package repl

import (
	"testing"
	"time"
)

// The context plans put from state kept across commands follows the command
// under way: live within one, cancelled by Interrupt, and unconstrained outside.
func TestHeldPlanContextFollowsTheCommandUnderWay(t *testing.T) {
	s := NewSession()
	held := s.command.scoped()
	if err := held.Err(); err != nil {
		t.Fatalf("outside a command: %v", err)
	}

	end := s.enter()
	if err := held.Err(); err != nil {
		t.Fatalf("within a command: %v", err)
	}
	s.Interrupt()
	select {
	case <-held.Done():
	default:
		t.Fatal("Interrupt did not cancel the held context")
	}
	end()

	if err := held.Err(); err != nil {
		t.Fatalf("after the interrupted command ended: %v", err)
	}
	end = s.enter()
	if err := held.Err(); err != nil {
		t.Fatalf("within the next command: %v", err)
	}
	end()
}

// An interrupt that arrives after a command was requested but before it took
// the session is kept for it; one with no command under way or waiting has
// nothing to stop and raises nothing.
func TestAnInterruptBeforeACommandBeginsIsKeptForIt(t *testing.T) {
	s := NewSession()
	s.Interrupt()
	if s.interrupt.Load() {
		t.Fatal("an interrupt with nothing to stop raised the flag")
	}

	release := s.reading()
	entered := make(chan func())
	go func() { entered <- s.enter() }()
	deadline := time.Now().Add(10 * time.Second)
	for waiting := 0; waiting == 0; {
		s.command.mu.Lock()
		waiting = s.command.waiting
		s.command.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("the command never waited for the session")
		}
		time.Sleep(time.Millisecond)
	}
	s.Interrupt()
	release()
	end := <-entered
	if !s.interrupt.Load() {
		t.Fatal("the interrupt before the command began was lost")
	}
	end()
	if s.interrupt.Load() {
		t.Fatal("the flag outlived the command it stopped")
	}
}
