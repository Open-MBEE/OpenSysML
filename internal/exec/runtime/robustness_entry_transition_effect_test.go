package runtime

import (
	"errors"
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

	t.Run("choice without an enabled branch disables entry", func(t *testing.T) {
		exec := stateExecutorForSource(t, "Machine", `package test {
			attribute def Go;
			attribute def Later;
			state Machine {
				attribute ready : Boolean = false;
				entry; then start;
				state start;
				state target {
					entry action boot { }
					transition boot then pick;
					choice pick;
					transition pick if ready then nested;
					state nested;
				}
				state fallback;
				transition first start accept Go then target;
				transition first start accept Later then fallback;
			}
		}`)
		exec.SendSignal("Go", nil)
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("ProcessNextEvent(Go) = %v, want disabled transition", err)
		}
		assertCurrentState(t, exec, "start")
		exec.SendSignal("Later", nil)
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("ProcessNextEvent(Later): %v", err)
		}
		assertCurrentState(t, exec, "fallback")
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
