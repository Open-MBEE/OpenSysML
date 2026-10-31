package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessRequirementAssumption covers the failure modes of judging a
// requirement as `allTrue(assumptions()) implies allTrue(constraints())`: an
// assumption that cannot be evaluated is the error, never a vacuous pass, and a
// denied requirement whose assumption fails is a violation naming the assumption.
func TestRuntimeRobustnessRequirementAssumption(t *testing.T) {
	const thing = `
		part def Thing { attribute enabled : Boolean; attribute level : Integer; }
		requirement def LevelOk {
			subject t : Thing;
			assume constraint { t.enabled }
			require constraint { t.level == 1 }
		}
		part off : Thing { attribute :>> enabled = false; attribute :>> level = 5; }
	`
	t.Run("an assumption over an unbound subject is the error, not a vacuous pass", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			private import ScalarValues::*;`+thing+`
			requirement unbound : LevelOk;
		}`)
		sym := requirementNamed(t, pkg, "unbound")
		result, err := ctx.CheckRequirementOn(sym, sym.OwnerScope, nil)
		if err == nil || result.Holds || result.Vacuous != "" {
			t.Fatalf("holds = %v, vacuous = %q, err = %v, want the unbound subject reported", result.Holds, result.Vacuous, err)
		}
		if !strings.Contains(err.Error(), "t subject is unbound") {
			t.Errorf("err = %v, want the unbound subject named", err)
		}
	})
	t.Run("a non-boolean assumption is the error, not a vacuous pass", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			private import ScalarValues::*;`+thing+`
			requirement def Loose { subject t : Thing; assume constraint { t.level } require constraint { false } }
			requirement loose : Loose { subject t = off; }
		}`)
		sym := requirementNamed(t, pkg, "loose")
		result, err := ctx.CheckRequirementOn(sym, sym.OwnerScope, nil)
		if err == nil || errors.Is(err, ErrViolated) || result.Holds {
			t.Fatalf("holds = %v, err = %v, want an evaluation error", result.Holds, err)
		}
		if !strings.Contains(err.Error(), "must evaluate to boolean") {
			t.Errorf("err = %v, want the non-boolean assumption reported", err)
		}
	})
	t.Run("an assumption declared after the required condition is judged first", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			private import ScalarValues::*;
			requirement r { require constraint { false } assume constraint { false } }
			assert not requirement nr { require constraint { false } assume constraint { false } }
			requirement stepped {
				attribute a : Real = 2.0;
				require constraint { assign a := a + 1; a < 0 }
				assume constraint { a > 10 }
			}
			requirement erring {
				attribute a : Real = 2.0;
				require constraint { a / 0 > 1 and 1 }
				assume constraint { false }
			}
		}`)
		for _, name := range []string{"r", "stepped", "erring"} {
			sym := requirementNamed(t, pkg, name)
			result, err := ctx.CheckRequirementOn(sym, sym.OwnerScope, nil)
			if err != nil || !result.Holds {
				t.Fatalf("%s: holds = %v, err = %v, want a vacuous pass", name, result.Holds, err)
			}
			if !strings.Contains(result.Vacuous, "vacuous: assumption") {
				t.Errorf("%s: vacuity %q, want the failed assumption named", name, result.Vacuous)
			}
		}
		sym := requirementNamed(t, pkg, "nr")
		result, err := ctx.CheckRequirementOn(sym, sym.OwnerScope, nil)
		var violation *ViolationError
		if !errors.As(err, &violation) || result.Holds {
			t.Fatalf("nr: holds = %v, err = %v, want a *ViolationError", result.Holds, err)
		}
		if !strings.Contains(violation.Condition, "assumption false of requirement nr did not hold") {
			t.Errorf("nr: violation names %q, want the assumption", violation.Condition)
		}
	})
	t.Run("a failed assumption makes the requirement hold vacuously, saying which", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			private import ScalarValues::*;`+thing+`
			requirement offReq : LevelOk { subject t = off; }
		}`)
		sym := requirementNamed(t, pkg, "offReq")
		result, err := ctx.CheckRequirementOn(sym, sym.OwnerScope, nil)
		if err != nil || !result.Holds {
			t.Fatalf("holds = %v, err = %v, want a vacuous pass", result.Holds, err)
		}
		if want := "vacuous: assumption t.enabled of requirement offReq did not hold"; !strings.Contains(result.Vacuous, want) {
			t.Errorf("vacuity %q, want %q", result.Vacuous, want)
		}
	})
	t.Run("denying a requirement whose assumption fails is a violation naming the assumption", func(t *testing.T) {
		ctx, pkg := conditionFixture(t, `package test {
			private import ScalarValues::*;`+thing+`
			assert not requirement nrOff : LevelOk { subject t = off; }
		}`)
		sym := requirementNamed(t, pkg, "nrOff")
		result, err := ctx.CheckRequirementOn(sym, sym.OwnerScope, nil)
		var violation *ViolationError
		if !errors.As(err, &violation) || result.Holds {
			t.Fatalf("holds = %v, err = %v, want a *ViolationError", result.Holds, err)
		}
		if want := "assumption t.enabled of requirement nrOff did not hold"; !strings.Contains(violation.Condition, want) {
			t.Errorf("violation names %q, want %q", violation.Condition, want)
		}
	})
}
