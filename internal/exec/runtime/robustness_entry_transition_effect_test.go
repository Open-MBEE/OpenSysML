package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

func TestRuntimeRobustnessEntryTransitionEffect(t *testing.T) {
	t.Run("effect error is returned", func(t *testing.T) {
		_, _, err := executeStateSource(t, "Machine", `package test {
			state Machine {
				attribute zero : Integer = 0;
				entry action boot { }
				transition boot do action {
					assign zero := 1 / zero;
				} then active;
				state active;
			}
		}`)
		if !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("execution error = %v, want ErrDivisionByZero", err)
		}
	})

	t.Run("junction is resolved after the entry action", func(t *testing.T) {
		_, _, err := executeStateSource(t, "Machine", `package test {
			state Machine {
				attribute ready : Boolean = true;
				entry action boot { assign ready := false; }
				transition boot then route;
				junction route;
				transition route if ready then active;
				state active;
			}
		}`)
		if !errors.Is(err, errNoWayThrough) {
			t.Fatalf("execution error = %v, want errNoWayThrough", err)
		}
	})

	t.Run("choice without an enabled branch after the effect returns its typed error", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			attribute def Go;
			state Machine {
				attribute ready : Boolean = false;
				attribute attempts : Integer = 0;
				entry; then start;
				state start;
				state target {
					entry action boot { }
					transition boot do action {
						assign attempts := attempts + 1;
					} then pick;
					choice pick;
					transition pick if ready then nested;
					state nested;
				}
				transition first start accept Go then target;
			}
		}`)
		exec.SendSignal("Go", nil)
		err := exec.ProcessNextEvent()
		if !errors.Is(err, ErrChoiceWithoutBranch) || !errors.Is(err, errNoWayThrough) {
			t.Fatalf("ProcessNextEvent(Go) = %v, want ErrChoiceWithoutBranch wrapped by errNoWayThrough", err)
		}
		if got := exec.StateData()["attempts"].Const.Int; got != 1 {
			t.Fatalf("entry effect attempts = %d, want 1 before resolving the choice", got)
		}
	})

	t.Run("junction without an enabled branch is resolved before the effect", func(t *testing.T) {
		err := stateExecutorError(t, `package test {
			state Machine {
				attribute ready : Boolean = false;
				entry action boot { }
				transition boot do action {
					assign ready := true;
				} then pick;
				junction pick;
				transition pick if ready then active;
				state active;
			}
		}`, "Machine")
		if !errors.Is(err, errNoWayThrough) || errors.Is(err, ErrChoiceWithoutBranch) {
			t.Fatalf("execution error = %v, want errNoWayThrough from junction pick", err)
		}
	})

	t.Run("nested default-entry junction without a way through fails when its entry is taken", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			attribute def Go;
			state Machine {
				entry; then idle;
				state idle;
				state blocked {
					entry action boot { }
					transition boot then route;
					junction route;
					transition route if false then never;
					state never;
				}
				transition first idle accept Go then blocked;
			}
		}`)
		exec.SendSignal("Go", nil)
		err := exec.ProcessNextEvent()
		if !errors.Is(err, errNoWayThrough) {
			t.Fatalf("ProcessNextEvent(Go) = %v, want errNoWayThrough", err)
		}
		if !strings.Contains(err.Error(), "junction route") {
			t.Fatalf("ProcessNextEvent(Go) = %v, want it to name junction route", err)
		}
	})

	t.Run("fork target remains unsupported", func(t *testing.T) {
		err := stateExecutorError(t, `package test {
			state Machine {
				entry; then split;
				fork split;
				state active;
			}
		}`, "Machine")
		var targetErr *lower.EntryTransitionTargetError
		if !errors.As(err, &targetErr) {
			t.Fatalf("executor error = %v, want EntryTransitionTargetError", err)
		}
	})

	t.Run("trigger remains unsupported", func(t *testing.T) {
		err := stateExecutorError(t, `package test {
			attribute def Go;
			state Machine {
				entry action boot { }
				transition boot accept Go then active;
				state active;
			}
		}`, "Machine")
		var shapeErr *lower.EntryTransitionShapeError
		if !errors.As(err, &shapeErr) {
			t.Fatalf("executor error = %v, want EntryTransitionShapeError", err)
		}
	})
}
