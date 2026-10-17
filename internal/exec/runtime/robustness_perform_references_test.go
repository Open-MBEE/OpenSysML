package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestRuntimeRobustnessPerformReferences covers perform statements and state subaction
// references that name nothing performable.
func TestRuntimeRobustnessPerformReferences(t *testing.T) {
	t.Run("perform_of_missing_action", testPerformOfMissingAction)
	t.Run("perform_reference_cycle", testPerformReferenceCycle)
	t.Run("state_subaction_reference_of_missing_action", testStateSubactionReferenceOfMissingAction)
	t.Run("state_subaction_reference_feature_chain", testStateSubactionReferenceFeatureChain)
}

// testStateSubactionReferenceOfMissingAction: an entry action given by
// reference to a name nothing declares fails at execution, naming the target.
func testStateSubactionReferenceOfMissingAction(t *testing.T) {
	ctx, machine := loadState(t, `package test {
		state Machine {
			entry; then init;
			state init;
			state active {
				entry noSuchAction;
			}

			succession first init then active;
			succession first active then done;
		}
	}`, "Machine")

	if _, _, err := ctx.ExecuteStateWithEvents(machine, nil); err == nil {
		t.Fatal("expected an unresolved entry action reference to fail")
	} else if !strings.Contains(err.Error(), "noSuchAction") {
		t.Errorf("error should name the unresolved action, got: %v", err)
	}
}

// testStateSubactionReferenceFeatureChain: a feature-chain reference performs
// the action as the object the chain denotes, so a part holding none names the
// chained action in its error rather than running the action as the machine.
func testStateSubactionReferenceFeatureChain(t *testing.T) {
	ctx, machine := loadState(t, `package test {
		action def CoolDown {
			first start;
			done;
			succession first start then done;
		}

		state Machine {
			part controller [0..1] {
				action coolDown : CoolDown;
			}

			entry; then init;
			state init;
			state active {
				exit controller.coolDown;
			}

			succession first init then active;
			succession first active then done;
		}
	}`, "Machine")

	_, _, err := ctx.ExecuteStateWithEvents(machine, nil)
	if !errors.Is(err, errPerformerNotObject) {
		t.Fatalf("expected errPerformerNotObject over an empty controller, got: %v", err)
	}
	if !strings.Contains(err.Error(), "controller.coolDown") {
		t.Errorf("error should name the chained action, got: %v", err)
	}
}

// testPerformOfMissingAction: a perform statement naming nothing resolvable is
// an error at execution, not a silently skipped node.
func testPerformOfMissingAction(t *testing.T) {
	ctx, outer := loadAction(t, `package test {
		action outer {
			first start;
			perform action doIt references missingAction;
			done;

			succession first start then doIt;
			succession first doIt then done;
		}
	}`, "outer")

	if _, err := ctx.ExecuteAction(outer); err == nil {
		t.Fatal("expected performing an unresolved action to fail")
	} else if !strings.Contains(err.Error(), "missingAction") {
		t.Errorf("error should name the unresolved action, got: %v", err)
	}
}

// testPerformReferenceCycle: an action performing itself must be stopped by the
// nesting bound instead of recursing forever.
func testPerformReferenceCycle(t *testing.T) {
	ctx, outer := loadAction(t, `package test {
		action outer {
			first start;
			perform action doIt references outer;
			done;

			succession first start then doIt;
			succession first doIt then done;
		}
	}`, "outer")

	done := make(chan error, 1)
	go func() {
		_, err := ctx.ExecuteAction(outer)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a self-performing action to be bounded, it completed")
		}
	case <-watchdog(10 * time.Second):
		t.Fatal("self-performing action did not terminate")
	}
}
