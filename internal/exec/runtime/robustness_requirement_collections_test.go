package runtime

import (
	"strings"
	"testing"
)

// TestRuntimeRobustnessRequirementCollections covers the failure modes of a
// requirement's `assumptions` and `constraints` collections (Requirements.sysml
// `RequirementCheck`), whose members are the objects of its assume and require
// constraints: an assumption whose expression cannot evaluate is the error, under
// the member's `result` and under the collection's invocation alike, never `false`,
// and whatever an earlier member read; one reading an unbound subject is undetermined; a requirement stating no
// assumption holds vacuously, `allTrue` over no member being true; and the
// collection takes no arguments.
func TestRuntimeRobustnessRequirementCollections(t *testing.T) {
	src := `package test {
		private import ScalarValues::Integer;
		private import ControlFunctions::allTrue;
		part def Vehicle { attribute fuelMass : Integer; attribute fuelFullMass : Integer; }
		part vehicleUnderTest : Vehicle { attribute redefines fuelMass = 200; attribute redefines fuelFullMass = 200; }
		requirement def FuelRequirement {
			subject vehicle : Vehicle;
			assume constraint nonnegativeFuel :> assumptions { vehicle.fuelMass >= 0 }
			require constraint fuelLimit :> constraints { vehicle.fuelMass <= 2000 }
		}
		requirement divides : FuelRequirement {
			subject vehicle : Vehicle = vehicleUnderTest;
			assume constraint perZero :> assumptions { vehicle.fuelMass / 0 > 0 }
		}
		requirement unbound : FuelRequirement;
		requirement def UnboundThenDivides {
			subject vehicle : Vehicle;
			assume constraint reads :> assumptions { vehicle.fuelMass >= 0 }
			assume constraint perZero :> assumptions { 1 / 0 > 0 }
		}
		requirement unboundThenDivides : UnboundThenDivides;
		requirement def NoAssumption {
			subject vehicle : Vehicle;
			require constraint fuelLimit :> constraints { vehicle.fuelMass <= 2000 }
		}
		requirement assumesNothing : NoAssumption { subject vehicle : Vehicle = vehicleUnderTest; }
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
		for _, expr := range []string{"perZero.result", "assumptions.result", "allTrue(assumptions())", "allTrue(assumptions.result)"} {
			if _, err := evalOn(t, ctx, req, expr); err == nil || !strings.Contains(err.Error(), "zero") {
				t.Errorf("%s: err = %v, want the division by zero", expr, err)
			}
		}
		if _, err := evalOn(t, ctx, req, "result"); err == nil || !strings.Contains(err.Error(), "zero") {
			t.Errorf("result: err = %v, want the division by zero", err)
		}
		if got := mustEvalOn(t, ctx, req, "constraints.result"); FormatValue(got) != "[true]" {
			t.Errorf("constraints.result = %s, want [true]", FormatValue(got))
		}
	})

	t.Run("unbound subject is undetermined, not false", func(t *testing.T) {
		req := instantiate(t, "test::unbound")
		for _, expr := range []string{"nonnegativeFuel.result", "assumptions.result", "allTrue(assumptions())", "constraints.result", "allTrue(constraints())", "result"} {
			if got := mustEvalOn(t, ctx, req, expr); got.Kind != ValUndetermined {
				t.Errorf("%s = %s, want undetermined", expr, FormatValue(got))
			}
		}
	})

	t.Run("an error after an undetermined member is the error", func(t *testing.T) {
		req := instantiate(t, "test::unboundThenDivides")
		if got := mustEvalOn(t, ctx, req, "reads.result"); got.Kind != ValUndetermined {
			t.Errorf("reads.result = %s, want undetermined", FormatValue(got))
		}
		for _, expr := range []string{"assumptions()", "allTrue(assumptions())", "assumptions.result"} {
			if _, err := evalOn(t, ctx, req, expr); err == nil || !strings.Contains(err.Error(), "zero") {
				t.Errorf("%s: err = %v, want the division by zero", expr, err)
			}
		}
	})

	t.Run("no assumption holds vacuously", func(t *testing.T) {
		req := instantiate(t, "test::assumesNothing")
		if got := mustEvalOn(t, ctx, req, "assumptions"); FormatValue(got) != "[]" {
			t.Errorf("assumptions = %s, want []", FormatValue(got))
		}
		for _, expr := range []string{"allTrue(assumptions())", "allTrue(constraints())", "result"} {
			if got := mustEvalOn(t, ctx, req, expr); got.Kind != ValConst || !got.Const.Bool {
				t.Errorf("%s = %s, want true", expr, FormatValue(got))
			}
		}
	})

	t.Run("collection takes no arguments", func(t *testing.T) {
		req := instantiate(t, "test::assumesNothing")
		if _, err := evalOn(t, ctx, req, "assumptions(1)"); err == nil {
			t.Error("assumptions(1): no error, want a refusal of the argument")
		}
	})
}
