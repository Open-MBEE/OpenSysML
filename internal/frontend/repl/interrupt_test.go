package repl

import (
	"testing"
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
