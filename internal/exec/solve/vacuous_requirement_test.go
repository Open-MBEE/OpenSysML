package solve

import (
	"errors"
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
	if _, err := ConstraintViolation(ctx, cn, cn.OwnerScope, nil); !errors.Is(err, ErrNoConditions) {
		t.Fatalf("a constraint assuming but requiring nothing: %v, want ErrNoConditions", err)
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

// TestAssumedConflictingConstraintStillRefuses: an assumption carrying a
// conflicting result expression is refused before vacuity is judged, as the
// evaluator refuses it, so neither the plain nor the violation query answers.
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
