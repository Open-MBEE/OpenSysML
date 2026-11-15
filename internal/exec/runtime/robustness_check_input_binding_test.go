package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// TestRuntimeRobustnessCheckInputBinding: a feature of the checked object spelled as an input or
// feature of the checked condition does not bind it; the unsupplied check is a typed error naming it.
func TestRuntimeRobustnessCheckInputBinding(t *testing.T) {
	t.Run("constraint input is not bound by the object's feature", testCheckInputBindingConstraint)
	t.Run("optional input binds empty beside the object's feature", testCheckInputBindingOptional)
	t.Run("requirement input is not bound by the object's feature", testCheckInputBindingRequirement)
	t.Run("requirement feature is not read from the satisfying object", testCheckInputBindingRequirementFeature)
	t.Run("typed check binds only the input its usage states", testCheckInputBindingTypedCheck)
	t.Run("typed case assertion binds only the input its usage states", testCheckInputBindingTypedAssertion)
}

const checkInputBindingModel = `package test {
	constraint def MassLimit { in m; in limit; m <= limit }
	requirement def Light { in limit; attribute m; require constraint { m <= limit } }
	part def Vehicle {
		attribute m = 1200;
		attribute limit = 1500;
		constraint mass : MassLimit;
		constraint none { in limit[0..1]; SequenceFunctions::isEmpty(limit) }
		constraint local { attribute limit; limit > 0 }
		requirement light : Light;
	}
	requirement def Slow { attribute speed; require constraint { speed <= 2.0 } }
	requirement slow : Slow;
	requirement slowBound : Slow { subject v : Lander; attribute :>> speed = v.speed; }
	part def Lander { attribute speed; }
	part lander : Lander { attribute :>> speed = 1.0; }
	part context {
		assert satisfy slow by lander;
		assert satisfy slowBound by lander;
	}
}`

func checkInputBindingVehicle(t *testing.T) (*Context, *symbols.Symbol, *Instance) {
	t.Helper()
	ctx, pkg := conditionFixture(t, checkInputBindingModel)
	vehicle := requirementNamed(t, pkg, "Vehicle")
	inst, err := ctx.Instantiate(vehicle)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return ctx, vehicle, inst
}

func testCheckInputBindingConstraint(t *testing.T) {
	ctx, vehicle, inst := checkInputBindingVehicle(t)
	feat := featureNamed(ctx, vehicle, "mass")
	if feat == nil {
		t.Fatal("constraint mass not found")
	}
	_, err := ctx.CheckConstraintWith(feat.Symbol, feat.DeclScope(), inst, CheckArgs{})
	if !errors.Is(err, errUnboundParameter) || !strings.Contains(err.Error(), `parameter "m"`) {
		t.Fatalf("no arguments: %v, want %v naming m", err, errUnboundParameter)
	}
	_, err = ctx.CheckConstraintWith(feat.Symbol, feat.DeclScope(), inst, CheckArgs{Named: map[string]Value{"m": integerValue(1200)}})
	if !errors.Is(err, errUnboundParameter) || !strings.Contains(err.Error(), `parameter "limit"`) {
		t.Fatalf("m alone: %v, want %v naming limit", err, errUnboundParameter)
	}
	result, err := ctx.CheckConstraintWith(feat.Symbol, feat.DeclScope(), inst, CheckArgs{Positional: []Value{integerValue(1600), integerValue(1500)}})
	if !errors.Is(err, ErrViolated) || result.Holds {
		t.Fatalf("m = 1600: holds %v, %v; want violated over the arguments, not the object's m", result.Holds, err)
	}
}

func testCheckInputBindingOptional(t *testing.T) {
	ctx, vehicle, inst := checkInputBindingVehicle(t)
	none := featureNamed(ctx, vehicle, "none")
	local := featureNamed(ctx, vehicle, "local")
	if none == nil || local == nil {
		t.Fatal("constraints none and local not found")
	}
	if result, err := ctx.CheckConstraintWith(none.Symbol, none.DeclScope(), inst, CheckArgs{}); err != nil || !result.Holds {
		t.Errorf("none: holds %v, %v; want the input its range leaves empty, not the object's limit", result.Holds, err)
	}
	if _, err := ctx.CheckConstraintWith(local.Symbol, local.DeclScope(), inst, CheckArgs{}); !errors.Is(err, ErrNoValue) {
		t.Errorf("local: %v, want %v for the constraint's own valueless limit", err, ErrNoValue)
	}
}

func testCheckInputBindingRequirement(t *testing.T) {
	ctx, vehicle, inst := checkInputBindingVehicle(t)
	feat := featureNamed(ctx, vehicle, "light")
	if feat == nil {
		t.Fatal("requirement light not found")
	}
	_, err := ctx.CheckRequirementWith(feat.Symbol, feat.DeclScope(), inst, CheckArgs{})
	if !errors.Is(err, errUnboundParameter) || !strings.Contains(err.Error(), `parameter "limit"`) {
		t.Fatalf("no arguments: %v, want %v naming limit", err, errUnboundParameter)
	}
	_, err = ctx.CheckRequirementWith(feat.Symbol, feat.DeclScope(), inst, CheckArgs{Named: map[string]Value{"limit": integerValue(1500)}})
	if !IsUninitializedFeatureValue(err) || !strings.HasSuffix(err.Error(), ": m") {
		t.Fatalf("limit alone: %v, want %v for the requirement's own m", err, errUninitializedFeatureValue)
	}
}

func testCheckInputBindingRequirementFeature(t *testing.T) {
	ctx, pkg := conditionFixture(t, checkInputBindingModel)
	contextSym := requirementNamed(t, pkg, "context")
	assertions := ctx.SatisfyAssertionsIn(contextSym.Scope)
	if len(assertions) != 2 {
		t.Fatalf("%d satisfy assertions, want 2", len(assertions))
	}
	for _, a := range assertions {
		satisfied, err := ctx.EvaluateSatisfaction(a)
		if strings.Contains(a.Text(), "slowBound") {
			if err != nil || !satisfied {
				t.Errorf("%s = %v, %v; want satisfied through the stated binding", a.Text(), satisfied, err)
			}
			continue
		}
		if !errors.Is(err, ErrNoValue) || !strings.Contains(err.Error(), "speed") || satisfied {
			t.Errorf("%s = %v, %v; want %v for the feature nothing binds", a.Text(), satisfied, err, ErrNoValue)
		}
	}
}

// checkInputBindingTypedModel types checks and case assertions by one definition whose input the
// owner spells as a feature of its own: stated bindings read it, the unbound usages do not.
const checkInputBindingTypedModel = `package test {
	private import ScalarValues::*;
	constraint def Limit { in x : Integer; x > 4 }
	part def Rig {
		attribute x : Integer = 7;
		constraint unbound : Limit;
		constraint bound : Limit { in x = Rig::x; }
		constraint low : Limit { in x = 3; }
		assert constraint assertedUnbound : Limit;
		assert constraint assertedBound : Limit { in x = Rig::x; }
	}
	analysis seq {
		attribute x : Integer := 0;
		action fill { assign x := 7; }
		then assert constraint unbound : Limit;
		then assert constraint bound : Limit { in x = x; }
		then assert constraint low : Limit { in x = 3; }
		return r : Integer = x;
	}
}`

func testCheckInputBindingTypedCheck(t *testing.T) {
	ctx, pkg := conditionFixture(t, checkInputBindingTypedModel)
	rig := requirementNamed(t, pkg, "Rig")
	inst, err := ctx.Instantiate(rig)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	check := func(name string) (CheckResult, error) {
		t.Helper()
		feat := featureNamed(ctx, rig, name)
		if feat == nil {
			t.Fatalf("constraint %s not found", name)
		}
		return ctx.CheckConstraintWith(feat.Symbol, feat.DeclScope(), inst, CheckArgs{})
	}
	for _, name := range []string{"unbound", "assertedUnbound"} {
		if _, err := check(name); !errors.Is(err, errUnboundParameter) || !strings.Contains(err.Error(), `parameter "x"`) {
			t.Errorf("%s: %v, want %v naming x: the rig's x of 7 does not bind it", name, err, errUnboundParameter)
		}
	}
	for _, name := range []string{"bound", "assertedBound"} {
		if result, err := check(name); err != nil || !result.Holds {
			t.Errorf("%s: holds %v, %v; want held over the rig's x its binding states", name, result.Holds, err)
		}
	}
	if result, err := check("low"); !errors.Is(err, ErrViolated) || result.Holds {
		t.Errorf("low: holds %v, %v; want violated over its binding of 3, not the rig's x", result.Holds, err)
	}
}

func testCheckInputBindingTypedAssertion(t *testing.T) {
	verdicts, err := caseAssertionVerdicts(t, checkInputBindingTypedModel, "test::seq")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if got, ok := verdicts["unbound"]; !ok || got.Status != VerdictUndecided || got.Detail == "" {
		t.Errorf("unbound = %v (%q, reported %v), want undecided: the case's x of 7 does not bind it", got.Status, got.Detail, ok)
	}
	if got := verdicts["bound"]; got.Status != VerdictSatisfied {
		t.Errorf("bound = %v (%q), want satisfied: its binding reads the case's x of 7", got.Status, got.Detail)
	}
	if got := verdicts["low"]; got.Status != VerdictNotSatisfied {
		t.Errorf("low = %v (%q), want not satisfied: its binding of 3 masks the case's x", got.Status, got.Detail)
	}
}
