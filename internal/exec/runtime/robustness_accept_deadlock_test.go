package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestRuntimeRobustnessAcceptDeadlock covers accepts that can never be satisfied and how the
// deadlock reports every waiting accept.
func TestRuntimeRobustnessAcceptDeadlock(t *testing.T) {
	t.Run("accept_deadlock_never_satisfied", testAcceptDeadlockNeverSatisfied)
	t.Run("accept_deadlock_reports_every_waiting_accept", testAcceptDeadlockReportsEveryWaitingAccept)
	t.Run("accept_statement_deadlock_in_a_loop", testAcceptStatementDeadlockInALoop)
}

// testAcceptDeadlockNeverSatisfied: an accept nothing can ever satisfy suspends
// the action, and a suspension that can never end must be reported as a typed
// deadlock rather than hanging.
func testAcceptDeadlockNeverSatisfied(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := executeActionSource(t, "pipeline", `package P {
			action pipeline {
				first start;
				action reader accept n : Integer;
				done;
				succession first start then reader;
				succession first reader then done;
			}
		}`)
		done <- err
	}()

	var err error
	select {
	case err = <-done:
	case <-watchdog(10 * time.Second):
		t.Fatal("an action waiting for a message that cannot arrive did not terminate")
	}

	if err == nil {
		t.Fatal("expected a deadlock error, the suspended accept completed")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
	for _, want := range []string{"accept n", "Integer"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in the deadlock report, got: %v", want, err)
		}
	}
}

// testAcceptDeadlockReportsEveryWaitingAccept: with two accepts parked in
// parallel branches and only one message in flight, the accept that can proceed
// does, and the report names the one still waiting rather than the whole action.
func testAcceptDeadlockReportsEveryWaitingAccept(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute got : Integer = 0;
			first start;
			action sender { send 7 to reader; }
			fork split;
			action reader accept n : Integer;
			action recorder { assign got := n; }
			action listener accept text : String;
			join sync;
			done;
			succession first start then sender;
			succession first sender then split;
			succession first split then reader;
			succession first split then listener;
			succession first reader then recorder;
			succession first recorder then sync;
			succession first listener then sync;
			succession first sync then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected a deadlock error: no String is ever sent")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock, got: %v", err)
	}
	if !strings.Contains(err.Error(), "accept text waiting since step 4 for a message of type String") {
		t.Errorf("expected the still-waiting accept in the report, got: %v", err)
	}
	if strings.Contains(err.Error(), "accept n ") {
		t.Errorf("the Integer accept was satisfied and must not be reported as waiting: %v", err)
	}
}

// testAcceptStatementDeadlockInALoop: a loop accept with no possible sender
// deadlocks with the parked accept named in the error.
func testAcceptStatementDeadlockInALoop(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := executeActionSource(t, "pipeline", `package P {
			action pipeline {
				first start;
				action waiter {
					loop {
						accept n : Integer;
					}
				}
				done;
				succession first start then waiter;
				succession first waiter then done;
			}
		}`)
		done <- err
	}()

	var err error
	select {
	case err = <-done:
	case <-watchdog(10 * time.Second):
		t.Fatal("a loop waiting for a message that cannot arrive did not terminate")
	}

	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("error = %v, want ErrAcceptDeadlock", err)
	}
	if !strings.Contains(err.Error(), "accept n") {
		t.Errorf("expected the parked accept in the error, got: %v", err)
	}
}
