package runtime

import (
	"errors"
	"testing"
)

// TestRuntimeRobustnessUsageValues: a performance through a usage reads the usage's
// own parameter values, so one that cannot be evaluated is the typed error of its
// expression, surfacing where the performance reads it.
func TestRuntimeRobustnessUsageValues(t *testing.T) {
	t.Run("state_usage_value_cannot_be_evaluated", testStateUsageUnevaluableValue)
	t.Run("subsetted_usage_value_cannot_be_evaluated", testSubsettedUsageUnevaluableValue)
}

func testStateUsageUnevaluableValue(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		state def M {
			in n;
			entry; then s1;
			state s1;
			state s2;
			transition first s1 if n > 2 then s2;
		}
		state m : M { in n = 1 / 0; }
	}`))
	_, _, err := ctx.ExecuteStateWithEvents(oneSymbol(t, idx, "test::m"), nil)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("ExecuteState(m) = %v, want ErrDivisionByZero from the usage's value", err)
	}
}

func testSubsettedUsageUnevaluableValue(t *testing.T) {
	err := runOuterAction(t, `package test {
		action def Step {
			in x;
			out y;
			first start;
			then assign y := x + 1;
		}
		action t : Step { in x = 1 / 0; }
		action outer {
			first start;
			then perform action pt : Step :> t;
		}
	}`)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("outer = %v, want ErrDivisionByZero from the subsetted usage's value", err)
	}
}
