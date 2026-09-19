package runtime

import (
	"errors"
	"testing"
)

// TestNegatedConstraintAssertion checks the verdict of `assert not constraint
// { … }`: the assertion holds when the condition it denies is false, and is
// violated when that condition holds (SysML v2 §8.3.21.10, Invariant::isNegated).
func TestNegatedConstraintAssertion(t *testing.T) {
	ctx, idx := contextForSource(t, `
		package test {
			part def Tank {
				attribute pressure = 40.0;
				assert not constraint { pressure > 100.0 }
				assert not constraint { pressure > 10.0 }
			}
		}
	`)
	tank := lookupOne(t, idx, "test::Tank")
	inst, err := ctx.Instantiate(tank)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	anonymous := tank.Scope.AnonymousMembers()
	if len(anonymous) != 2 {
		t.Fatalf("found %d anonymous members, want the 2 negated assertions", len(anonymous))
	}

	holds, err := ctx.EvaluateConstraintOn(anonymous[0], tank.Scope, inst)
	if err != nil {
		t.Fatalf("EvaluateConstraintOn(denied condition is false): %v", err)
	}
	if !holds {
		t.Error("a negated assertion of a false condition must hold")
	}

	holds, err = ctx.EvaluateConstraintOn(anonymous[1], tank.Scope, inst)
	if holds {
		t.Error("a negated assertion of a true condition must not hold")
	}
	if !errors.Is(err, ErrViolated) {
		t.Errorf("error = %v, want ErrViolated", err)
	}
}
