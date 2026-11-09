package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessStateEntryAccept: an entry or do accept nothing sends to leaves its state
// active rather than ending the run; an exit accept nothing can post is a deadlock (exit runs at the instant).
func TestRuntimeRobustnessStateEntryAccept(t *testing.T) {
	t.Run("entry_accept_with_no_sender_stays_parked", testEntryAcceptWithNoSenderStaysParked)
	t.Run("do_accept_with_no_sender_stays_parked", testDoAcceptWithNoSenderStaysParked)
	t.Run("exit_accept_with_no_sender_deadlocks", testExitAcceptWithNoSenderDeadlocks)
}

const stateAcceptNoSender = `package P {
	attribute def Sig;
	state def M {
		entry; then s;
		state s {
			%s accept a : Sig;
		}
		transition first s then done;
		state done;
	}
}`

func testEntryAcceptWithNoSenderStaysParked(t *testing.T) {
	var visits []string
	err := runWithWatchdog(t, func() (err error) {
		_, visits, err = executeStateSource(t, "M", strings.ReplaceAll(stateAcceptNoSender, "%s", "entry"))
		return err
	})
	if err != nil {
		t.Fatalf("error = %v, want the run to end with the entry parked", err)
	}
	if len(visits) != 1 || visits[0] != "s" {
		t.Fatalf("stateVisits = %v, want [s]: the completion transition must wait for the entry accept", visits)
	}
}

func testDoAcceptWithNoSenderStaysParked(t *testing.T) {
	var visits []string
	err := runWithWatchdog(t, func() (err error) {
		_, visits, err = executeStateSource(t, "M", strings.ReplaceAll(stateAcceptNoSender, "%s", "do"))
		return err
	})
	if err != nil {
		t.Fatalf("error = %v, want the run to end with the do behavior parked", err)
	}
	if len(visits) != 1 || visits[0] != "s" {
		t.Fatalf("stateVisits = %v, want [s]", visits)
	}
}

func testExitAcceptWithNoSenderDeadlocks(t *testing.T) {
	err := runWithWatchdog(t, func() error {
		_, _, err := executeStateSource(t, "M", strings.ReplaceAll(stateAcceptNoSender, "%s", "exit"))
		return err
	})
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("error = %v, want ErrAcceptDeadlock", err)
	}
	if !strings.Contains(err.Error(), "exit action") {
		t.Fatalf("error = %v, want it to name the exit action", err)
	}
}
