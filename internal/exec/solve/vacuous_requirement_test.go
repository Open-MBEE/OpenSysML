package solve

import (
	"errors"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"strings"
	"testing"
)

// vacuousRequirementFixture states a requirement assuming but requiring nothing,
// one stating nothing at all, and a constraint assuming but requiring nothing.
func vacuousRequirementFixture(t *testing.T) (*Query, *Query, *Query, error) {
	t.Helper()
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::Integer;
			part def Rig {
				attribute level : Integer;
				requirement onlyAssumed { assume constraint { level > 1 } }
				requirement prose { doc /* words only */ }
				assert constraint cn { assume constraint { level > 1 } }
			}
		}
	`)
	assumed := symbolNamed(t, idx, "test::Rig::onlyAssumed")
	plain, err := Requirement(ctx, assumed, assumed.OwnerScope)
	if err != nil {
		return nil, nil, nil, err
	}
	violation, err := RequirementViolation(ctx, assumed, assumed.OwnerScope, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	prose := symbolNamed(t, idx, "test::Rig::prose")
	proseViolation, err := RequirementViolation(ctx, prose, prose.OwnerScope, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	cn := symbolNamed(t, idx, "test::Rig::cn")
	if _, err := ConstraintViolation(ctx, cn, cn.OwnerScope, nil); !errors.Is(err, runtime.ErrNoValue) {
		t.Fatalf("a constraint assuming but requiring nothing: %v, want ErrNoValue", err)
	}
	return plain, violation, proseViolation, nil
}

// TestAssumptionOnlyRequirementHoldsVacuously: a requirement's assumptions are
// the antecedent of its claim (`allTrue(assumptions()) implies
// allTrue(constraints())`), never asserted on their own, so one requiring
// nothing asserts nothing and every assignment meets it, as the evaluator
// judges it; its violation query asserts false, so it is unsatisfiable.
func TestAssumptionOnlyRequirementHoldsVacuously(t *testing.T) {
	plain, violation, proseViolation, err := vacuousRequirementFixture(t)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(plain.Assertions) != 0 || len(plain.Vars) != 0 {
		t.Errorf("the plain query asserts the assumption:\n%s", Script(plain))
	}
	for name, q := range map[string]*Query{"onlyAssumed": violation, "prose": proseViolation} {
		if len(q.Assertions) != 1 || writeTerm(q.Assertions[0].Term) != "(not true)" {
			t.Errorf("%s: the violation of a vacuous claim is not false:\n%s", name, Script(q))
			continue
		}
		if q.Assertions[0].From.Role != RoleViolated || q.Assertions[0].From.Condition != "not (true)" {
			t.Errorf("%s: violated assertion is %s %q", name, q.Assertions[0].From.Role, q.Assertions[0].From.Condition)
		}
	}
}

// TestAssumptionOnlyRequirementSolverVerdicts: over a solver, the plain query
// is satisfiable and the violation query unsatisfiable, so `%check` proves
// what `%requirement` observes.
func TestAssumptionOnlyRequirementSolverVerdicts(t *testing.T) {
	solver := requireSolver(t)
	plain, violation, _, err := vacuousRequirementFixture(t)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	solved(t, solver, plain, StatusSat)
	solved(t, solver, violation, StatusUnsat)
}

// TestAssumedConflictingConstraintStillRefuses: an assumed conflicting result
// expression is refused before vacuity is judged, as the evaluator refuses it.
func TestAssumedConflictingConstraintStillRefuses(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			constraint def A { true }
			constraint def B { false }
			constraint def C :> A, B;
			requirement r { assume constraint c : C; }
		}
	`)
	r := symbolNamed(t, idx, "test::r")
	for name, translate := range map[string]func() (*Query, error){
		"plain":     func() (*Query, error) { return Requirement(ctx, r, r.OwnerScope) },
		"violation": func() (*Query, error) { return RequirementViolation(ctx, r, r.OwnerScope, nil) },
	} {
		_, err := translate()
		if !errors.Is(err, ErrNotTranslatable) || !strings.Contains(err.Error(), "conflicting result expression") {
			t.Errorf("%s: %v, want a refusal for the conflicting result expression", name, err)
		}
	}
}

// TestDeniedVacuousRequirementIgnoresItsAssumptions: a requirement requiring nothing
// holds whatever its assumptions read, so its denial is false and that denial's violation true.
func TestDeniedVacuousRequirementIgnoresItsAssumptions(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			public import SI::*;
			part def Rig {
				attribute speed : ISQSpaceTime::SpeedValue;
				assert not requirement deniedFalse { assume constraint { false } }
				assert not requirement deniedOpaque { assume constraint { speed <= 5.4 [kg] } }
			}
		}
	`)
	for _, name := range []string{"deniedFalse", "deniedOpaque"} {
		sym := symbolNamed(t, idx, "test::Rig::"+name)
		denial, err := Requirement(ctx, sym, sym.OwnerScope)
		if err != nil {
			t.Fatalf("%s: translate the denial: %v", name, err)
		}
		if len(denial.Assertions) != 1 || writeTerm(denial.Assertions[0].Term) != "(not true)" {
			t.Errorf("%s: the denial of a vacuous claim is not false:\n%s", name, Script(denial))
		}
		violation, err := RequirementViolation(ctx, sym, sym.OwnerScope, nil)
		if err != nil {
			t.Fatalf("%s: translate the violation: %v", name, err)
		}
		if len(violation.Assertions) != 1 || writeTerm(violation.Assertions[0].Term) != "true" || violation.Assertions[0].From.Condition != "not (not (true))" {
			t.Errorf("%s: the violation of a denied vacuous claim is not true:\n%s", name, Script(violation))
		}
		x, err := NewTranslator(ctx, subjectOf(sym, "requirement", false))
		if err != nil {
			t.Fatalf("%s: translator: %v", name, err)
		}
		expr, err := x.Conditions(ctx.ConditionsOf(sym, sym.OwnerScope))
		if err != nil {
			t.Fatalf("%s: conditions: %v", name, err)
		}
		if writeTerm(expr.Term) != "(not true)" || len(expr.Defined) != 0 {
			t.Errorf("%s: the denied vacuous claim as an expression is %s, defined under %d conditions, want (not true) under none", name, writeTerm(expr.Term), len(expr.Defined))
		}
	}
}
