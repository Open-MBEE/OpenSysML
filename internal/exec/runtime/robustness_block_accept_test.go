package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessBlockAccept covers the failure modes of an accept node in
// a loop or branch body: a body parked at one with nothing left to send to it is
// a deadlock, and a run that keeps the body parked while another spins ends on
// the step budget rather than hanging.
func TestRuntimeRobustnessBlockAccept(t *testing.T) {
	t.Run("parked_body_with_no_sender_deadlocks", testParkedBodyWithNoSenderDeadlocks)
	t.Run("parked_body_in_branch_with_no_sender_deadlocks", testParkedBranchWithNoSenderDeadlocks)
	t.Run("step_budget_runs_out_while_parked", testStepBudgetRunsOutWhileParked)
	t.Run("eval_budget_runs_out_while_parked", testEvalBudgetRunsOutWhileParked)
}

// runWithWatchdog runs an action, failing the test if it does not return in time.
func runWithWatchdog(t *testing.T, run func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- run() }()
	select {
	case err := <-done:
		return err
	case <-watchdog(10 * time.Second):
		t.Fatal("the run did not terminate")
		return nil
	}
}

func testParkedBodyWithNoSenderDeadlocks(t *testing.T) {
	err := runWithWatchdog(t, func() error {
		_, err := executeActionSource(t, "waits", `package P {
			private import ScalarValues::*;
			action waits {
				attribute total : Integer = 0;
				first start;
				action reader {
					while total < 5 {
						accept n : Integer;
						assign total := total + n;
					}
				}
				done;
				succession first start then reader;
				succession first reader then done;
			}
		}`)
		return err
	})
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("error = %v, want ErrAcceptDeadlock", err)
	}
	if !strings.Contains(err.Error(), "accept n") {
		t.Errorf("expected the parked accept in the error, got: %v", err)
	}
}

func testParkedBranchWithNoSenderDeadlocks(t *testing.T) {
	err := runWithWatchdog(t, func() error {
		_, err := executeActionSource(t, "waits", `package P {
			private import ScalarValues::*;
			action waits {
				attribute total : Integer = 0;
				first start;
				fork split;
				action reader {
					if total == 0 {
						if true {
							accept n : Integer;
							assign total := total + n;
						}
					}
				}
				action sender { send 1 to nobody; }
				join sync;
				done;
				succession first start then split;
				succession first split then reader;
				succession first split then sender;
				succession first reader then sync;
				succession first sender then sync;
				succession first sync then done;
			}
		}`)
		return err
	})
	if err == nil {
		t.Fatal("expected an error: nothing sends to the accept two blocks deep")
	}
	if !errors.Is(err, ErrAcceptDeadlock) && !errors.Is(err, ErrSendTargetNotObject) && !strings.Contains(err.Error(), "nobody") {
		t.Fatalf("error = %v, want a typed deadlock or unroutable-send error", err)
	}
}

const parkedReaderSpinningSender = `package P {
	private import ScalarValues::*;
	action spins {
		attribute total : Integer = 0;
		attribute i : Integer = 0;
		first start;
		fork split;
		action reader {
			while true {
				accept n : Integer;
				assign total := total + n;
			}
		}
		action sender {
			first start;
			action bump { assign i := i + 1; }
			action again;
			succession first start then bump;
			succession first bump then again;
			succession first again then bump;
		}
		join sync;
		done;
		succession first start then split;
		succession first split then reader;
		succession first split then sender;
		succession first reader then sync;
		succession first sender then sync;
		succession first sync then done;
	}
}`

func testStepBudgetRunsOutWhileParked(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, parkedReaderSpinningSender))
	ctx.maxActionSteps = 100
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "spins", ast.DefAction)
	if sym == nil {
		t.Fatal("action spins not found")
	}
	err := runWithWatchdog(t, func() error {
		_, err := ctx.ExecuteAction(sym)
		return err
	})
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("error = %v, want ErrActionStepLimitExceeded", err)
	}
}

func testEvalBudgetRunsOutWhileParked(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, parkedReaderSpinningSender))
	ctx.maxSteps = 500
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "spins", ast.DefAction)
	if sym == nil {
		t.Fatal("action spins not found")
	}
	err := runWithWatchdog(t, func() error {
		_, err := ctx.ExecuteAction(sym)
		return err
	})
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("error = %v, want ErrStepLimitExceeded", err)
	}
}
