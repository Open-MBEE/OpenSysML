package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessVacuousRequirement covers the edges of a requirement check
// over no required constraint: the denial of one is a violation, a requirement
// stating only an assumption holds, two requirements requiring each other are a
// cycle rather than a vacuous pass, a required condition failing before a
// self-reference is still the violation it is, an assumption that cannot be read
// does not keep a requirement without constraint from holding, and a constraint
// stating nothing still has no verdict. Each is a typed error or a verdict, never a panic.
func TestRuntimeRobustnessVacuousRequirement(t *testing.T) {
	t.Run("denying_a_requirement_without_conditions", testDenyingARequirementWithoutConditions)
	t.Run("assumption_only_requirement", testAssumptionOnlyRequirement)
	t.Run("requirements_requiring_each_other", testRequirementsRequiringEachOther)
	t.Run("violation_before_a_self_reference", testViolationBeforeASelfReference)
	t.Run("unreadable_assumption_without_constraint", testUnreadableAssumptionWithoutConstraint)
	t.Run("constraint_without_conditions_stays_undecided", testConstraintWithoutConditionsStaysUndecided)
}

func testDenyingARequirementWithoutConditions(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			requirement def Prose { doc /* stated in prose only */ }
			requirement prose : Prose;
			part def Rig {
				assert not requirement denied : Prose;
			}
		}
	`)
	prose := requirementNamed(t, pkg, "prose")
	result, err := ctx.CheckRequirementOn(prose, prose.OwnerScope, nil)
	if err != nil || !result.Holds || !strings.HasPrefix(result.Vacuous, "vacuous: requirement prose") {
		t.Fatalf("prose: holds = %v, vacuity %q, err = %v", result.Holds, result.Vacuous, err)
	}
	rig := requirementNamed(t, pkg, "Rig")
	denied := featureNamed(ctx, rig, "denied")
	if denied == nil || denied.Symbol == nil {
		t.Fatal("requirement denied not found")
	}
	holds, err := ctx.EvaluateRequirementOn(denied.Symbol, denied.DeclScope(), nil)
	var violation *ViolationError
	if holds || !errors.As(err, &violation) {
		t.Fatalf("denied: holds = %v, err = %v, want a violation", holds, err)
	}
	if !strings.Contains(violation.Condition, "requirement denied states no required constraint, so it holds vacuously") {
		t.Errorf("the violation names %q, want the vacuous claim", violation.Condition)
	}
}

func testAssumptionOnlyRequirement(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			part def Craft { attribute ready = true; }
			requirement def Assumed {
				subject craft : Craft;
				assume constraint { craft.ready }
			}
			requirement assumed : Assumed;
			part craft : Craft;
			part context {
				assert satisfy assumed by craft;
				assert not satisfy assumed by craft;
			}
		}
	`)
	context := requirementNamed(t, pkg, "context")
	assertions := ctx.SatisfyAssertionsIn(context.Scope)
	if len(assertions) != 2 {
		t.Fatalf("found %d satisfaction assertions, want 2", len(assertions))
	}
	result, err := ctx.CheckSatisfactionOn(assertions[0], nil)
	if err != nil || !result.Holds || !strings.Contains(result.Vacuous, "requirement assumed states no required constraint") {
		t.Fatalf("satisfy assumed: holds = %v, vacuity %q, err = %v", result.Holds, result.Vacuous, err)
	}
	if satisfied, err := ctx.EvaluateSatisfaction(assertions[1]); satisfied || !errors.Is(err, ErrViolated) {
		t.Fatalf("not satisfy assumed: satisfied = %v, err = %v, want ErrViolated", satisfied, err)
	}
}

func testRequirementsRequiringEachOther(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			requirement def Wagon;
			requirement alpha { require beta; }
			requirement beta { require alpha; }
			part context {
				assert satisfy alpha by context;
			}
		}
	`)
	first := requirementNamed(t, pkg, "alpha")
	holds, err := ctx.EvaluateRequirementOn(first, first.OwnerScope, nil)
	if holds || !errors.Is(err, ErrNoConditions) {
		t.Fatalf("alpha: holds = %v, err = %v, want ErrNoConditions", holds, err)
	}
	if !strings.Contains(err.Error(), "require alpha names the requirement being checked") {
		t.Errorf("the error does not name the cycle: %v", err)
	}
	context := requirementNamed(t, pkg, "context")
	assertions := ctx.SatisfyAssertionsIn(context.Scope)
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}
	if satisfied, err := ctx.EvaluateSatisfaction(assertions[0]); satisfied || !errors.Is(err, ErrNoConditions) {
		t.Fatalf("satisfy alpha: satisfied = %v, err = %v, want ErrNoConditions", satisfied, err)
	}
}

func testConstraintWithoutConditionsStaysUndecided(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			constraint def Empty;
			constraint empty : Empty;
			constraint denied = not empty;
		}
	`)
	for _, name := range []string{"empty", "Empty"} {
		sym := requirementNamed(t, pkg, name)
		if holds, err := ctx.EvaluateConstraintOn(sym, sym.OwnerScope, nil); holds || !errors.Is(err, ErrNoConditions) {
			t.Errorf("%s: holds = %v, err = %v, want ErrNoConditions", name, holds, err)
		}
	}
}

func testViolationBeforeASelfReference(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			requirement cyclic {
				require constraint { false }
				require cyclic;
			}
		}
	`)
	cyclic := requirementNamed(t, pkg, "cyclic")
	holds, err := ctx.EvaluateRequirementOn(cyclic, cyclic.OwnerScope, nil)
	var violation *ViolationError
	if holds || !errors.As(err, &violation) || violation.Condition != "false" {
		t.Fatalf("cyclic: holds = %v, err = %v, want the violation at false", holds, err)
	}
}

func testUnreadableAssumptionWithoutConstraint(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			part def Craft { attribute ready : Boolean; }
			requirement def Assumed {
				subject craft : Craft;
				assume constraint { craft.ready }
			}
			requirement assumed : Assumed;
		}
	`)
	assumed := requirementNamed(t, pkg, "assumed")
	result, err := ctx.CheckRequirementOn(assumed, assumed.OwnerScope, nil)
	if err != nil || !result.Holds || !strings.Contains(result.Vacuous, "requirement assumed states no required constraint") {
		t.Fatalf("assumed: holds = %v, vacuity %q, err = %v, want a vacuous pass", result.Holds, result.Vacuous, err)
	}
}
