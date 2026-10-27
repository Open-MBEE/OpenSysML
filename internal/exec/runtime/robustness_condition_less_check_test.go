package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessConditionLessCheck: a constraint declaring no condition binds
// no result (KerML 1.1 §7.4.8.2); a requirement requiring nothing holds vacuously.
func TestRuntimeRobustnessConditionLessCheck(t *testing.T) {
	t.Run("a constraint definition declaring no condition has no value", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			constraint def Empty;
		}`)
		sym := requirementNamed(t, pkg, "Empty")
		holds, err := ctx.EvaluateConstraintOn(sym, sym.OwnerScope, nil)
		if holds || !errors.Is(err, ErrNoValue) || !strings.Contains(err.Error(), "the constraint declares no condition") {
			t.Fatalf("holds = %v, err = %v, want ErrNoValue naming the missing condition", holds, err)
		}
	})
	t.Run("a constraint usage declaring no condition has no value", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			constraint def Empty;
			constraint empty : Empty;
		}`)
		sym := requirementNamed(t, pkg, "empty")
		holds, err := ctx.EvaluateConstraintOn(sym, sym.OwnerScope, nil)
		if holds || !errors.Is(err, ErrNoValue) || !strings.Contains(err.Error(), "the constraint declares no condition") {
			t.Fatalf("holds = %v, err = %v, want ErrNoValue naming the missing condition", holds, err)
		}
	})
	t.Run("a return the body never binds carries the calc's hint", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			private import ScalarValues::*;
			constraint c { attribute r : Boolean = true; return r; }
		}`)
		sym := requirementNamed(t, pkg, "c")
		holds, err := ctx.EvaluateConstraintOn(sym, sym.OwnerScope, nil)
		if holds || !errors.Is(err, ErrNoValue) || !strings.Contains(err.Error(), "result parameter r binds no value") {
			t.Fatalf("holds = %v, err = %v, want ErrNoValue with the unbound-result hint", holds, err)
		}
	})
	t.Run("a requirement declaring no require or assume constraint holds vacuously", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			requirement def Bare;
			requirement bare : Bare;
		}`)
		for _, name := range []string{"Bare", "bare"} {
			sym := requirementNamed(t, pkg, name)
			if holds, err := ctx.EvaluateRequirementOn(sym, sym.OwnerScope, nil); !holds || err != nil {
				t.Errorf("%s: holds = %v, err = %v, want vacuously satisfied", name, holds, err)
			}
		}
	})
	t.Run("an assumed subrequirement requiring nothing stays vacuous", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			requirement outer {
				requirement r0;
				assume r0;
			}
		}`)
		sym := requirementNamed(t, pkg, "outer")
		if holds, err := ctx.EvaluateRequirementOn(sym, sym.OwnerScope, nil); !holds || err != nil {
			t.Fatalf("holds = %v, err = %v, want vacuously satisfied", holds, err)
		}
	})
}
