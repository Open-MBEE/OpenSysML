package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// TestRuntimeRobustnessTerminate exercises the failure modes of `terminate`:
// a target that is not an action, one no ongoing performance answers to, and
// a termination that must cancel paused work without reporting a deadlock.
func TestRuntimeRobustnessTerminate(t *testing.T) {
	t.Run("target_that_is_not_an_action", testTerminateTargetThatIsNotAnAction)
	t.Run("target_already_completed", testTerminateTargetAlreadyCompleted)
	t.Run("target_never_performed_here", testTerminateTargetNeverPerformedHere)
	t.Run("target_unresolved", testTerminateTargetUnresolved)
	t.Run("root_terminated_while_a_branch_waits", testTerminateRootWhileABranchWaits)
	t.Run("node_terminated_while_its_own_flow_waits", testTerminateNodeWhileItsOwnFlowWaits)
	t.Run("statements_after_terminate_do_not_run", testTerminateStopsFollowingStatements)
}

// runActionWithLibraries executes the named action of src over the standard
// library, failing the test if the run hangs.
func runActionWithLibraries(t *testing.T, src, name string) (map[string]Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), name, ast.DefAction)
	if sym == nil {
		t.Fatalf("action %s not found", name)
	}
	type result struct {
		outputs map[string]Value
		err     error
	}
	done := make(chan result, 1)
	go func() {
		outputs, err := ctx.ExecuteAction(sym)
		done <- result{outputs, err}
	}()
	select {
	case r := <-done:
		return r.outputs, r.err
	case <-time.After(10 * time.Second):
		t.Fatalf("action %s did not terminate", name)
		return nil, nil
	}
}

// testTerminateTargetThatIsNotAnAction: `terminate q` naming an attribute is a
// typed error, not a silent no-op.
func testTerminateTargetThatIsNotAnAction(t *testing.T) {
	_, err := runActionWithLibraries(t, `package test {
		private import ScalarValues::*;
		action def A {
			attribute q : Integer := 0;
			first start;
			action a { terminate q; }
			done;
			succession first start then a;
			succession first a then done;
		}
	}`, "A")
	if !errors.Is(err, ErrTerminateTarget) || !strings.Contains(err.Error(), "q is not an action") {
		t.Fatalf("error = %v, want ErrTerminateTarget naming q", err)
	}
}

// testTerminateTargetAlreadyCompleted: a node that has already performed is no
// longer ongoing, so terminating it is refused.
func testTerminateTargetAlreadyCompleted(t *testing.T) {
	_, err := runActionWithLibraries(t, `package test {
		private import ScalarValues::*;
		action def A {
			out x : Integer := 0;
			first start;
			action a { assign x := 1; }
			action b { terminate a; }
			done;
			succession first start then a;
			succession first a then b;
			succession first b then done;
		}
	}`, "A")
	if !errors.Is(err, ErrTerminateTarget) || !strings.Contains(err.Error(), "a is not being performed here") {
		t.Fatalf("error = %v, want ErrTerminateTarget naming a", err)
	}
}

// testTerminateTargetNeverPerformedHere: an action declared elsewhere and not
// performed by this run cannot be terminated from it.
func testTerminateTargetNeverPerformedHere(t *testing.T) {
	_, err := runActionWithLibraries(t, `package test {
		private import ScalarValues::*;
		action def Other { out y : Integer := 0; }
		action def A {
			first start;
			action a { terminate Other; }
			done;
			succession first start then a;
			succession first a then done;
		}
	}`, "A")
	if !errors.Is(err, ErrTerminateTarget) || !strings.Contains(err.Error(), "Other is not being performed here") {
		t.Fatalf("error = %v, want ErrTerminateTarget naming Other", err)
	}
}

// testTerminateTargetUnresolved: a target no declaration answers to is an
// unresolved reference.
func testTerminateTargetUnresolved(t *testing.T) {
	_, err := runActionWithLibraries(t, `package test {
		action def A {
			first start;
			action a { terminate nowhere; }
			done;
			succession first start then a;
			succession first a then done;
		}
	}`, "A")
	if !errors.Is(err, ErrUnresolvedReference) || !strings.Contains(err.Error(), "nowhere") {
		t.Fatalf("error = %v, want ErrUnresolvedReference naming nowhere", err)
	}
}

// testTerminateRootWhileABranchWaits: terminating the whole action while a
// concurrent branch waits on the clock ends the run cleanly — the wait is
// cancelled rather than reported as an abandoned, deadlocked run.
func testTerminateRootWhileABranchWaits(t *testing.T) {
	outputs, err := runActionWithLibraries(t, `package test {
		private import ScalarValues::*;
		action def A {
			out x : Integer := 0;
			out y : Integer := 0;
			first start;
			fork f;
			action a { assign x := 1; terminate A; assign x := 8; }
			action bw accept after 5 [SI::s];
			action b { assign y := 2; }
			join j;
			done;
			succession first start then f;
			succession first f then a;
			succession first f then bw;
			succession first bw then b;
			succession first a then j;
			succession first b then j;
			succession first j then done;
		}
	}`, "A")
	if err != nil {
		t.Fatalf("terminate A: %v", err)
	}
	assertIntOutput(t, outputs, "x", 1)
	assertIntOutput(t, outputs, "y", 0)
}

// testTerminateNodeWhileItsOwnFlowWaits: terminating a sibling whose nested
// flow waits on the clock cancels that wait; the join then completes without
// a deadlock and nothing of the sibling runs afterwards.
func testTerminateNodeWhileItsOwnFlowWaits(t *testing.T) {
	outputs, err := runActionWithLibraries(t, `package test {
		private import ScalarValues::*;
		action def A {
			out x : Integer := 0;
			out y : Integer := 0;
			first start;
			fork f;
			action aw accept after 1 [SI::s];
			action a { terminate b; assign x := 1; }
			action b {
				first start;
				action bw accept after 5 [SI::s];
				action bset { assign y := 2; }
				done;
				succession first start then bw;
				succession first bw then bset;
				succession first bset then done;
			}
			join j;
			done;
			succession first start then f;
			succession first f then aw;
			succession first aw then a;
			succession first f then b;
			succession first a then j;
			succession first b then j;
			succession first j then done;
		}
	}`, "A")
	if err != nil {
		t.Fatalf("terminate b: %v", err)
	}
	assertIntOutput(t, outputs, "x", 1)
	assertIntOutput(t, outputs, "y", 0)
}

// testTerminateStopsFollowingStatements: the statements after a `terminate`,
// in the body and in the loop around it, never run.
func testTerminateStopsFollowingStatements(t *testing.T) {
	outputs, err := runActionWithLibraries(t, `package test {
		private import ScalarValues::*;
		action def A {
			out n : Integer := 0;
			first start;
			action count {
				while n < 10 {
					assign n := n + 1;
					if n == 2 { terminate; }
				}
				assign n := 99;
			}
			done;
			succession first start then count;
			succession first count then done;
		}
	}`, "A")
	if err != nil {
		t.Fatalf("terminate in a loop: %v", err)
	}
	assertIntOutput(t, outputs, "n", 2)
}
