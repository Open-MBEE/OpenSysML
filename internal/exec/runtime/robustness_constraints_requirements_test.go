package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessConstraintsRequirements covers constraints, requirements and satisfy
// statements whose subjects or features cannot be resolved.
func TestRuntimeRobustnessConstraintsRequirements(t *testing.T) {
	t.Run("constraint_missing_feature", testConstraintMissingFeature)
	t.Run("nested_condition_subject_is_ambiguous", testNestedConditionSubjectIsAmbiguous)
	t.Run("satisfaction_subject_is_ambiguous", testSatisfactionSubjectIsAmbiguous)
	t.Run("recursive_composition_subject_search", testRecursiveCompositionSubjectSearch)
	t.Run("requirement_feature_without_a_value", testRequirementFeatureWithoutAValue)
	t.Run("requirement_features_valued_from_each_other", testRequirementFeaturesValuedFromEachOther)
	t.Run("satisfy_unresolved_requirement", testSatisfyUnresolvedRequirement)
	t.Run("satisfy_requirement_without_conditions", testSatisfyRequirementWithoutConditions)
	t.Run("satisfy_bounded_by_the_step_budget", testSatisfyBoundedByTheStepBudget)
}

// testSatisfyUnresolvedRequirement: a satisfaction assertion whose requirement
// reference names nothing reports it, rather than evaluating the assertion's own
// empty body as a verdict about the model.
func testSatisfyUnresolvedRequirement(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part lander;
			part context {
				assert satisfy nosuch by lander;
			}
		}
	`))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}

	satisfied, err := ctx.EvaluateSatisfaction(assertions[0])
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrNoRequirement) {
		t.Errorf("expected ErrNoRequirement, got: %v", err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("an unresolved requirement reference is not a violation")
	}
	if !strings.Contains(err.Error(), "nosuch") {
		t.Errorf("error does not name the reference: %v", err)
	}
}

// testSatisfyRequirementWithoutConditions: a requirement that states no condition
// is a requirement check over no constraint, which is true, so satisfying it holds
// and denying it is a violation — a verdict, never an evaluation failure.
func testSatisfyRequirementWithoutConditions(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			requirement def Documented {
				doc /* stated in prose only */
			}
			requirement documented : Documented;
			part lander;
			part context {
				assert satisfy documented by lander;
				assert not satisfy documented by lander;
			}
		}
	`))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 2 {
		t.Fatalf("found %d satisfaction assertions, want 2", len(assertions))
	}

	result, err := ctx.CheckSatisfactionOn(assertions[0], nil)
	if err != nil || !result.Holds {
		t.Fatalf("satisfy: holds = %v, err = %v, want a vacuous pass", result.Holds, err)
	}
	if want := "vacuous: requirement documented states no required constraint"; !strings.Contains(result.Vacuous, want) {
		t.Errorf("vacuity %q, want %q", result.Vacuous, want)
	}

	satisfied, err := ctx.EvaluateSatisfaction(assertions[1])
	if satisfied || !errors.Is(err, ErrViolated) {
		t.Fatalf("not satisfy: satisfied = %v, err = %v, want ErrViolated", satisfied, err)
	}
	if !strings.Contains(err.Error(), "states no required constraint, so it holds vacuously") {
		t.Errorf("the violation does not say why the requirement holds: %v", err)
	}
}

// testSatisfyBoundedByTheStepBudget: a satisfaction check is one run, so its
// condition evaluation spends the run's budget instead of resetting it.
func testSatisfyBoundedByTheStepBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Lander { attribute verticalSpeed = 1.2; }
			part lander : Lander;
			requirement def TouchdownRequirement {
				subject craft : Lander;
				attribute maxVerticalSpeed = 1.5;
				require constraint { craft.verticalSpeed <= maxVerticalSpeed }
			}
			requirement touchdown : TouchdownRequirement;
			part context {
				assert satisfy touchdown by lander;
			}
		}
	`))
	assertions := ctx.SatisfyAssertionsIn(idx.DocumentRoot("<test>"))
	if len(assertions) != 1 {
		t.Fatalf("found %d satisfaction assertions, want 1", len(assertions))
	}
	ctx.maxSteps = 2

	satisfied, err := ctx.EvaluateSatisfaction(assertions[0])
	if err == nil {
		t.Fatalf("expected the step budget to bound the check, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("an exhausted budget is not a verdict about the model")
	}

	// The subject is built inside the same run, so a budget exhausted there is
	// still reported as such rather than as a missing subject.
	ctx.maxSteps = 0
	if _, err := ctx.EvaluateSatisfaction(assertions[0]); !errors.Is(err, ErrStepLimitExceeded) || !errors.Is(err, errNoSubject) {
		t.Errorf("expected ErrStepLimitExceeded while building the subject, got: %v", err)
	}
}

// testConstraintMissingFeature: constraint references nonexistent feature
func testConstraintMissingFeature(t *testing.T) {
	src := `
		package test {
			constraint broken {
				nonexistent > 0 // 'nonexistent' feature doesn't exist
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}

	idx, model, ctx := buildRuntime(t, "<test>", file)

	_ = model // silence unused

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "broken", ast.DefConstraint)
	if sym == nil {
		t.Fatal("broken constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, rootScope)

	if err != nil {
		t.Logf("EvaluateConstraint returned error (expected): %v", err)
		return
	}

	if !satisfied {
		t.Log("EvaluateConstraint returned false (missing feature treated as unsatisfied)")
		return
	}

	t.Log("EvaluateConstraint returned true (missing feature tolerated)")
}

// testNestedConditionSubjectIsAmbiguous: two objects redefining the same nested
// feature differently make the subject of a check a question, reported as
// ErrAmbiguousSubject rather than answered from whichever object is found first.
func testNestedConditionSubjectIsAmbiguous(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				constraint small { value < 10.0 }
			}
			part def Top {
				part leaf : Leaf;
			}
			part slow : Top {
				part :>> leaf { attribute :>> value = 2.0; }
			}
			part fast : Top {
				part :>> leaf { attribute :>> value = 99.0; }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	for _, name := range []string{"slow", "fast"} {
		if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", name)); err != nil {
			t.Fatalf("instantiate %s: %v", name, err)
		}
	}
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	satisfied, err := ctx.EvaluateConstraint(small, small.OwnerScope)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("satisfied = %t, err = %v, want ErrAmbiguousSubject", satisfied, err)
	}
	if satisfied {
		t.Error("an ambiguous subject is no verdict")
	}
}

// testSatisfactionSubjectIsAmbiguous: a satisfaction assertion whose `by` object
// holds two objects of the requirement's owner has no one subject either, and
// reports it as ErrAmbiguousSubject rather than picking one.
func testSatisfactionSubjectIsAmbiguous(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				requirement lim { require value < 10.0; }
			}
			part def Top {
				part slow : Leaf { attribute :>> value = 2.0; }
				part fast : Leaf { attribute :>> value = 99.0; }
			}
			part top : Top;
			assert satisfy Leaf::lim by top;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	pkg := memberPath(t, rootScope, "test")
	assertions := ctx.SatisfyAssertionsIn(pkg.Scope)
	if len(assertions) != 1 {
		t.Fatalf("assertions = %d, want the one the package states", len(assertions))
	}
	result, err := ctx.CheckSatisfactionOn(assertions[0], nil)
	if !errors.Is(err, ErrAmbiguousSubject) {
		t.Fatalf("holds = %t, err = %v, want ErrAmbiguousSubject", result.Holds, err)
	}
	if result.Holds {
		t.Error("an ambiguous subject is no verdict")
	}
}

// testRecursiveCompositionSubjectSearch: searching for the object a check is
// about does not walk a design containing its own kind forever; it answers about
// the declaration, since no object of the checked type is there.
func testRecursiveCompositionSubjectSearch(t *testing.T) {
	src := `
		package test {
			part def Leaf {
				attribute value = 1.0;
				constraint small { value < 10.0 }
			}
			part def Node {
				part next : Node;
			}
			part root : Node;
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	if _, err := ctx.Instantiate(memberPath(t, rootScope, "test", "root")); err != nil {
		t.Fatalf("instantiate root: %v", err)
	}
	small := memberPath(t, rootScope, "test", "Leaf", "small")
	done := make(chan struct{})
	var satisfied bool
	var err error
	go func() {
		defer close(done)
		satisfied, err = ctx.EvaluateConstraint(small, small.OwnerScope)
	}()
	select {
	case <-done:
	case <-watchdog(20 * time.Second):
		t.Fatal("the subject search did not terminate on recursive composition")
	}
	if err != nil {
		t.Fatalf("EvaluateConstraint: %v", err)
	}
	if !satisfied {
		t.Error("satisfied = false, want the declaration's answer")
	}
	if len(ctx.instances) > 1000 {
		t.Errorf("%d objects materialized: the search is not bounded", len(ctx.instances))
	}
}

// testRequirementFeatureWithoutAValue: a condition naming a feature the
// requirement declares but nothing gives a value to reports ErrNoValue, naming
// the feature, rather than the unresolved-feature error of a name that is not
// declared at all.
func testRequirementFeatureWithoutAValue(t *testing.T) {
	src := `
		package test {
			requirement def TouchdownRequirement {
				attribute actualVerticalSpeed;
				attribute maxVerticalSpeed = 1.5;
				require constraint { actualVerticalSpeed <= maxVerticalSpeed }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "TouchdownRequirement", ast.DefRequirement)
	if sym == nil {
		t.Fatal("TouchdownRequirement not found")
	}

	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("a feature without a value is not a violation")
	}
	if !strings.Contains(err.Error(), "actualVerticalSpeed") {
		t.Errorf("error does not name the feature: %v", err)
	}
}

// testRequirementFeaturesValuedFromEachOther: two features whose values name each
// other report a cycle promptly instead of recursing until the step budget runs out.
func testRequirementFeaturesValuedFromEachOther(t *testing.T) {
	src := `
		package test {
			requirement def R {
				attribute a = b;
				attribute b = a;
				require constraint { a <= b }
			}
		}
	`
	file := parseAndBuild(t, src)
	if file == nil {
		t.Fatal("parse failed")
	}
	idx, _, ctx := buildRuntime(t, "<test>", file)
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "R", ast.DefRequirement)
	if sym == nil {
		t.Fatal("R not found")
	}

	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if err == nil {
		t.Fatalf("expected an error, got satisfied = %v", satisfied)
	}
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Errorf("expected ErrCyclicFeatureValue, got: %v", err)
	}
}
