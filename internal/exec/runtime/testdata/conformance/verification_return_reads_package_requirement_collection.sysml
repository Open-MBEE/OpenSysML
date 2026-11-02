// A verification's returned expression reads the `result` of a package-level requirement's
// `assumptions` collection through the object the usage would materialize: `[]` while
// nothing fills the collection, so `allTrue` of it holds.
package test {
	private import ScalarValues::Integer;
	private import ControlFunctions::allTrue;
	private import VerificationCases::VerdictKind;
	private import VerificationCases::PassIf;

	part def Vehicle { attribute fuelMass : Integer; }
	part vehicleUnderTest : Vehicle { attribute redefines fuelMass = -200; }

	requirement def FuelRequirement {
		subject vehicle : Vehicle;
		assume constraint nonnegativeFuel { vehicle.fuelMass >= 0 }
		require constraint { vehicle.fuelMass <= 2000 }
	}
	requirement fullTankRequirement : FuelRequirement {
		subject vehicle : Vehicle = vehicleUnderTest;
	}

	verification check {
		subject vehicle = vehicleUnderTest;
		objective { verify fullTankRequirement; }
		return verdict : VerdictKind =
			if allTrue(fullTankRequirement.assumptions.result) ? PassIf(fullTankRequirement()) else VerdictKind::inconclusive;
	}
}
