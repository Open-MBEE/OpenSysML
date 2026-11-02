package runtime

import (
	"slices"
	"testing"
)

const requirementConstraintSubsettingModel = `package test {
	private import ScalarValues::Integer;
	part def Vehicle { attribute fuelMass : Integer; attribute fuelFullMass : Integer; }
	part vehicleUnderTest : Vehicle { attribute redefines fuelMass = -200; attribute redefines fuelFullMass = -200; }
	requirement def FuelRequirement {
		subject vehicle : Vehicle;
		assume constraint nonnegativeFuel { vehicle.fuelMass >= 0 }
		require constraint maxFuel { vehicle.fuelMass <= 2000 }
	}
	requirement fullTankRequirement : FuelRequirement {
		subject vehicle : Vehicle = vehicleUnderTest;
		assume constraint fullTank { vehicle.fuelMass == vehicle.fuelFullMass }
	}
}`

// A requirement's `assume`/`require` constraints subset the RequirementCheck
// collections their membership kind names (SysML v2 8.3.20.4), the inherited
// ones of its definition included, so the runtime sees them among the
// collections' subsetters; the verdicts they hold stay where they were.
func TestRequirementConstraintsSubsetTheRequirementCheckCollections(t *testing.T) {
	ctx, idx := libraryShapeContext(t, requirementConstraintSubsettingModel)
	instantiateQualified(t, ctx, idx, "test::vehicleUnderTest")
	req := instantiateQualified(t, ctx, idx, "test::fullTankRequirement")
	typ := lookupOne(t, idx, "test::fullTankRequirement")

	for path, want := range map[string]string{
		"test::fullTankRequirement::fullTank":    "assumptions",
		"test::FuelRequirement::nonnegativeFuel": "assumptions",
		"test::FuelRequirement::maxFuel":         "constraints",
	} {
		if got := ctx.subsettedNamesOf(req, lookupOne(t, idx, path)); !slices.Contains(got, want) {
			t.Errorf("subsettedNamesOf(%s) = %v, want %s among them", path, got, want)
		}
	}
	for collection, want := range map[string][]string{"assumptions": {"fullTank", "nonnegativeFuel"}, "constraints": {"maxFuel"}} {
		got := featureNames(ctx.SubsettingFeatures(req, typ, collection))
		for _, name := range want {
			if !slices.Contains(got, name) {
				t.Errorf("SubsettingFeatures(%s) = %v, want %s among them", collection, got, name)
			}
		}
	}
	for path, want := range map[string]bool{"test::fullTankRequirement::fullTank": true, "test::FuelRequirement::nonnegativeFuel": false} {
		sym := lookupOne(t, idx, path)
		// A violated assertion is reported as an error beside its false verdict.
		holds, err := ctx.EvaluateConstraintOn(sym, sym.OwnerScope, req)
		if got := err == nil && holds; got != want {
			t.Errorf("EvaluateConstraintOn(%s) = %v, %v, want holds %v", path, holds, err, want)
		}
	}
}
