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
