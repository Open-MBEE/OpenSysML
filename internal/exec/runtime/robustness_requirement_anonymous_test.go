package runtime

import (
	"strings"
	"testing"
)

// TestRuntimeRobustnessAnonymousRequirementConstraints: an anonymous `assume`/`require` member of
// `assumptions`/`constraints` keeps an error as the error, an unbound subject undetermined, a violation `false`.
func TestRuntimeRobustnessAnonymousRequirementConstraints(t *testing.T) {
	src := `package test {
		private import ScalarValues::Integer;
		private import ControlFunctions::allTrue;
		part def Vehicle { attribute fuelMass : Integer; attribute fuelFullMass : Integer; }
		part vehicleUnderTest : Vehicle { attribute redefines fuelMass = 2500; attribute redefines fuelFullMass = 2500; }
		requirement def FuelRequirement {
			subject vehicle : Vehicle;
			assume constraint { vehicle.fuelMass >= 0 }
			require constraint { vehicle.fuelMass <= 2000 }
		}
		requirement divides : FuelRequirement {
			subject vehicle : Vehicle = vehicleUnderTest;
			assume constraint { vehicle.fuelMass / 0 > 0 }
		}
		requirement unbound : FuelRequirement {
			require constraint { vehicle.fuelMass == vehicle.fuelFullMass }
		}
		requirement overfull : FuelRequirement {
			subject vehicle : Vehicle = vehicleUnderTest;
			assume constraint { vehicle.fuelMass == vehicle.fuelFullMass }
		}
	}`
	ctx, idx := libraryShapeContext(t, src)
	instantiate := func(t *testing.T, name string) *Instance {
		t.Helper()
		inst, err := ctx.Instantiate(oneSymbol(t, idx, name))
		if err != nil {
			t.Fatalf("instantiate %s: %v", name, err)
		}
		return inst
	}

	t.Run("evaluation error is the error, not false", func(t *testing.T) {
		req := instantiate(t, "test::divides")
		for _, expr := range []string{"assumptions.result", "allTrue(assumptions())", "allTrue(assumptions.result)", "result"} {
			if _, err := evalOn(t, ctx, req, expr); err == nil || !strings.Contains(err.Error(), "zero") {
				t.Errorf("%s: err = %v, want the division by zero", expr, err)
			}
		}
		if got := mustEvalOn(t, ctx, req, "constraints.result"); FormatValue(got) != "[false]" {
			t.Errorf("constraints.result = %s, want [false]", FormatValue(got))
		}
	})

	t.Run("unbound subject is undetermined, not false", func(t *testing.T) {
		req := instantiate(t, "test::unbound")
		for _, expr := range []string{"assumptions.result", "allTrue(assumptions())", "constraints.result", "allTrue(constraints())", "result"} {
			if got := mustEvalOn(t, ctx, req, expr); got.Kind != ValUndetermined {
				t.Errorf("%s = %s, want undetermined", expr, FormatValue(got))
			}
		}
	})

	t.Run("violated inherited constraint is false, not absent", func(t *testing.T) {
		req := instantiate(t, "test::overfull")
		if got := mustEvalOn(t, ctx, req, "assumptions.result"); FormatValue(got) != "[true, true]" {
			t.Errorf("assumptions.result = %s, want [true, true]", FormatValue(got))
		}
		if got := mustEvalOn(t, ctx, req, "constraints.result"); FormatValue(got) != "[false]" {
			t.Errorf("constraints.result = %s, want [false]", FormatValue(got))
		}
		for _, expr := range []string{"allTrue(constraints())", "result"} {
			if got := mustEvalOn(t, ctx, req, expr); got.Kind != ValConst || got.Const.Bool {
				t.Errorf("%s = %s, want false", expr, FormatValue(got))
			}
		}
	})
}
