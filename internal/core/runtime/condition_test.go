package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// conditionFixture loads src and returns the runtime context and the scope of
// its `test` package.
func conditionFixture(t *testing.T, src string) (*Context, *symbols.Scope) {
	t.Helper()
	file := parser.New(source.New("test.sysml", []byte(src))).ParseFile()
	idx := libs.NewModelIndex()
	idx.AddDocument("test.sysml", file)
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
	return ctx, idx.DocumentRoot("test.sysml").Children()[0]
}

func requirementNamed(t *testing.T, scope *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := scope.LookupLocal(name)
	if !ok {
		t.Fatalf("%s not found", name)
	}
	return sym
}

// A requirement's conditions see its own attributes, whether the requirement
// states them itself or inherits them from the definition it is typed by.
func TestRequirementConditionSeesOwnAttributes(t *testing.T) {
	src := `
		package test {
			private import RealFunctions::*;
			requirement def TouchdownRequirement {
				attribute actualVerticalSpeed;
				attribute maxVerticalSpeed;
				require constraint {
					actualVerticalSpeed <= maxVerticalSpeed
				}
			}
			requirement own {
				attribute a = -1.2;
				attribute b = 1.5;
				require constraint { abs(a) <= b }
			}
			requirement touchdown : TouchdownRequirement {
				attribute :>> actualVerticalSpeed = 1.2;
				attribute :>> maxVerticalSpeed = 1.5;
			}
			requirement hardLanding : TouchdownRequirement {
				attribute :>> actualVerticalSpeed = 2.4;
				attribute :>> maxVerticalSpeed = 1.5;
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)

	for _, name := range []string{"own", "touchdown"} {
		satisfied, err := ctx.EvaluateRequirement(requirementNamed(t, pkg, name), pkg)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !satisfied {
			t.Errorf("%s: not satisfied", name)
		}
	}

	satisfied, err := ctx.EvaluateRequirement(requirementNamed(t, pkg, "hardLanding"), pkg)
	if !errors.Is(err, ErrViolated) {
		t.Fatalf("hardLanding: err = %v, want ErrViolated", err)
	}
	if satisfied {
		t.Error("hardLanding reported as satisfied")
	}
	// The verdict names the condition that failed, not only the requirement.
	var violation *ViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("hardLanding: err = %v, want a *ViolationError", err)
	}
	if violation.Condition != "actualVerticalSpeed <= maxVerticalSpeed" {
		t.Errorf("condition = %q, want the failed comparison", violation.Condition)
	}
}

// An attribute a requirement declares without a value has no value to check
// against, which is distinct from naming something that does not exist.
func TestRequirementConditionWithoutValueIsNotUnresolved(t *testing.T) {
	src := `
		package test {
			private import RealFunctions::*;
			requirement def TouchdownRequirement {
				attribute actualVerticalSpeed;
				attribute maxVerticalSpeed;
				require constraint {
					actualVerticalSpeed <= maxVerticalSpeed
				}
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	_, err := ctx.EvaluateRequirement(requirementNamed(t, pkg, "TouchdownRequirement"), pkg)
	if !errors.Is(err, ErrNoValue) {
		t.Fatalf("err = %v, want ErrNoValue", err)
	}
	if !strings.Contains(err.Error(), "actualVerticalSpeed") {
		t.Errorf("err = %v, want the feature named", err)
	}
}

// A constraint checked at model level over a feature the model leaves open is not
// decided: ErrNoValue naming the feature, never a verdict; a constant operand still decides.
func TestConstraintOverUndeterminedOperandIsNotDecided(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			attribute u : Real;
			constraint big { u > 3.0 }
			constraint fixed { (u > 3.0) or true }
		}
	`
	ctx, pkg := conditionFixture(t, src)
	_, err := ctx.EvaluateConstraint(requirementNamed(t, pkg, "big"), pkg)
	if !errors.Is(err, ErrNoValue) {
		t.Fatalf("big: err = %v, want ErrNoValue", err)
	}
	if !strings.Contains(err.Error(), "u has no value in the model") {
		t.Errorf("big: err = %v, want the open feature named", err)
	}
	holds, err := ctx.EvaluateConstraint(requirementNamed(t, pkg, "fixed"), pkg)
	if err != nil || !holds {
		t.Errorf("fixed = %v, %v; want true without error", holds, err)
	}
}

// An assumption that does not hold is not a violation: assumptions are trusted.
func TestRequirementAssumptionIsNotRequired(t *testing.T) {
	src := `
		package test {
			requirement pessimistic {
				attribute a = 2.0;
				assume constraint { a <= 1.0 }
				require constraint { a <= 3.0 }
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	satisfied, err := ctx.EvaluateRequirement(requirementNamed(t, pkg, "pessimistic"), pkg)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if !satisfied {
		t.Error("a false assumption should not violate the requirement")
	}
}

// A requirement stating only assumptions has nothing that can be violated, as
// for a constraint (see conformance case constraint_assume).
func TestRequirementWithOnlyAssumptionsIsSatisfied(t *testing.T) {
	src := `
		package test {
			requirement assumed {
				attribute a = 2.0;
				assume constraint { a <= 3.0 }
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	satisfied, err := ctx.EvaluateRequirement(requirementNamed(t, pkg, "assumed"), pkg)
	if err != nil {
		t.Fatalf("evaluation failed: %v", err)
	}
	if !satisfied {
		t.Error("an assumption that holds is not a violation")
	}
}

// A negated element stating only assumptions has nothing to deny, so it is not a
// verdict either way.
func TestNegatedConstraintWithOnlyAssumptionsIsNotAVerdict(t *testing.T) {
	src := `
		package test {
			part def Rig {
				attribute a = 2.0;
				assert not constraint cn { assume constraint { a > 1.0 } }
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	rig := requirementNamed(t, pkg, "Rig")
	inst, err := ctx.Instantiate(rig)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	feat := featureNamed(ctx, rig, "cn")
	if feat == nil || feat.Symbol == nil {
		t.Fatal("constraint feature cn not found")
	}
	satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
	if !errors.Is(err, ErrNoConditions) {
		t.Errorf("expected ErrNoConditions, got: %v", err)
	}
	if satisfied {
		t.Error("an assumption alone is not a verdict about a negated element")
	}
}

// A `not` written on a nested constraint inverts the single condition of its body.
func TestNegatedNestedConstraintIsInverted(t *testing.T) {
	src := `
		package test {
			constraint def C {
				in a;
				assert not constraint { a > 100 }
			}
			part def Rig {
				attribute a = 1.0;
				constraint holds : C { in a = 1.0; }
				constraint fails : C { in a = 200.0; }
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	rig := requirementNamed(t, pkg, "Rig")
	inst, err := ctx.Instantiate(rig)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for name, want := range map[string]bool{"holds": true, "fails": false} {
		feat := featureNamed(ctx, rig, name)
		if feat == nil || feat.Symbol == nil {
			t.Fatalf("%s: constraint feature not found", name)
		}
		satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if satisfied != want {
			t.Errorf("%s: satisfied = %v, want %v", name, satisfied, want)
		}
		// The violation names the condition as written, negation included.
		var violation *ViolationError
		if !want && errors.As(err, &violation) && violation.Condition != "not a > 100" {
			t.Errorf("%s: condition = %q, want %q", name, violation.Condition, "not a > 100")
		}
	}
}

// A `not` on a body of several conditions negates their conjunction, so it holds
// as soon as one of them fails.
func TestNegatedNestedConstraintNegatesTheConjunction(t *testing.T) {
	src := `
		package test {
			constraint def C {
				in a;
				in b;
				assert not constraint {
					a > 100
					b > 100
				}
			}
			part def Rig {
				constraint oneFails : C { in a = 200.0; in b = 1.0; }
				constraint bothHold : C { in a = 200.0; in b = 200.0; }
				constraint neitherHolds : C { in a = 1.0; in b = 1.0; }
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	rig := requirementNamed(t, pkg, "Rig")
	inst, err := ctx.Instantiate(rig)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	for name, want := range map[string]bool{"oneFails": true, "bothHold": false, "neitherHolds": true} {
		feat := featureNamed(ctx, rig, name)
		if feat == nil || feat.Symbol == nil {
			t.Fatalf("%s: constraint feature not found", name)
		}
		satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if satisfied != want {
			t.Errorf("%s: satisfied = %v, want %v", name, satisfied, want)
		}
		// The violation names the denied conjunction, not one of its conditions.
		var violation *ViolationError
		if !want && errors.As(err, &violation) && violation.Condition != "not { a > 100; b > 100 }" {
			t.Errorf("%s: condition = %q, want %q", name, violation.Condition, "not { a > 100; b > 100 }")
		}
	}
}

// A parameter bound to a same-named outer feature reads that outer feature
// rather than resolving to itself.
func TestParameterBoundToSameNamedFeature(t *testing.T) {
	src := `
		package test {
			constraint def MassLimit {
				in mass;
				mass <= 1500.0
			}
			part def Vehicle {
				attribute mass = 1200.0;
				constraint light : MassLimit { in mass = mass; }
			}
			part def Truck {
				attribute mass = 4000.0;
				constraint heavy : MassLimit { in mass = mass; }
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	for _, tc := range []struct {
		part, constraint string
		want             bool
	}{
		{"Vehicle", "light", true},
		{"Truck", "heavy", false},
	} {
		owner := requirementNamed(t, pkg, tc.part)
		inst, err := ctx.Instantiate(owner)
		if err != nil {
			t.Fatalf("%s: Instantiate: %v", tc.part, err)
		}
		feat := featureNamed(ctx, owner, tc.constraint)
		if feat == nil || feat.Symbol == nil {
			t.Fatalf("%s: constraint feature not found", tc.constraint)
		}
		satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Errorf("%s: %v", tc.constraint, err)
			continue
		}
		if satisfied != tc.want {
			t.Errorf("%s: satisfied = %v, want %v", tc.constraint, satisfied, tc.want)
		}
	}
}

// An inherited parameter nothing binds still reads a same-named value of the
// object being checked.
func TestUnboundParameterFallsBackToInstanceFeatureValue(t *testing.T) {
	src := `
		package test {
			constraint def MassLimit {
				in m;
				in limit;
				m <= limit
			}
			part def Vehicle {
				attribute m = 1200.0;
				attribute limit = 1500.0;
				constraint mass : MassLimit;
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	vehicle := requirementNamed(t, pkg, "Vehicle")
	inst, err := ctx.Instantiate(vehicle)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	feat := featureNamed(ctx, vehicle, "mass")
	if feat == nil || feat.Symbol == nil {
		t.Fatal("constraint feature not found")
	}
	satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
	if err != nil {
		t.Fatalf("EvaluateConstraintOn: %v", err)
	}
	if !satisfied {
		t.Error("an unbound parameter should read the checked object's value")
	}
}

// A parameter a constraint usage binds is visible to the condition it inherits
// from the definition it is typed by.
func TestConstraintUsageBindsInheritedParameter(t *testing.T) {
	src := `
		package test {
			constraint def MassLimit {
				in m;
				in limit;
				m <= limit
			}
			part def Vehicle {
				attribute mass = 1200.0;
				constraint withinLimit : MassLimit {
					in m = mass;
					in limit = 1500.0;
				}
				constraint overLimit : MassLimit {
					in m = mass;
					in limit = 1000.0;
				}
			}
		}
	`
	ctx, pkg := conditionFixture(t, src)
	vehicle := requirementNamed(t, pkg, "Vehicle")
	inst, err := ctx.Instantiate(vehicle)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}

	for name, want := range map[string]bool{"withinLimit": true, "overLimit": false} {
		feat := featureNamed(ctx, vehicle, name)
		if feat == nil || feat.Symbol == nil {
			t.Fatalf("%s: constraint feature not found", name)
		}
		satisfied, err := ctx.EvaluateConstraintOn(feat.Symbol, feat.DeclScope(), inst)
		if err != nil && !errors.Is(err, ErrViolated) {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if satisfied != want {
			t.Errorf("%s: satisfied = %v, want %v", name, satisfied, want)
		}
	}
}

// A violated assertion over quantities names its operands as they were written,
// bracketed unit and all, rather than as the index expression they parse into.
func TestViolationRendersQuantityOperands(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			constraint tooShort {
				1.0 [m] > 500.0 [m]
			}
			constraint tooSlow {
				2.0 [km] / 1.0 [s] < 30.0 [m/s]
			}
		}
	`))
	pkg := idx.DocumentRoot("<test>").Children()[0]

	for name, want := range map[string]string{
		"tooShort": "1.0 [m] > 500.0 [m]",
		"tooSlow":  "2.0 [km] / 1.0 [s] < 30.0 [m/s]",
	} {
		satisfied, err := ctx.EvaluateConstraint(requirementNamed(t, pkg, name), pkg)
		if !errors.Is(err, ErrViolated) {
			t.Fatalf("%s: satisfied = %v, err = %v; want ErrViolated", name, satisfied, err)
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v; want it to render the condition as %q", name, err, want)
		}
	}
}

// A condition stated inside a require body reads the names that body declares,
// which live in the body's own scope rather than the enclosing element's.
func TestRequireBodyConditionReadsABodyLocalName(t *testing.T) {
	ctx, pkg := conditionFixture(t, `
		package test {
			requirement def Limit { attribute cap; }
			requirement lim : Limit { attribute :>> cap = 5; }
			requirement study {
				require lim {
					attribute margin = 2;
					require constraint { margin > 1 }
				}
			}
		}
	`)
	satisfied, err := ctx.EvaluateRequirement(requirementNamed(t, pkg, "study"), pkg)
	if err != nil {
		t.Fatalf("study: %v", err)
	}
	if !satisfied {
		t.Error("study: not satisfied")
	}
}
