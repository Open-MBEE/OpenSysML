// A verification's returned expression reads a package-level requirement usage as the
// object the usage would materialize, as a chain read does: calling the requirement's
// `assumptions` collection on it evaluates its assumed constraints on that object —
// `nonnegativeFuel` fails for a fuel mass of -200, so the case is inconclusive.
package test {
	private import ScalarValues::Integer;
	private import ControlFunctions::allTrue;
	private import VerificationCases::VerdictKind;
	private import VerificationCases::PassIf;

	part def Vehicle { attribute fuelMass : Integer; attribute fuelFullMass : Integer; }
	part vehicleUnderTest : Vehicle { attribute redefines fuelMass = -200; attribute redefines fuelFullMass = -200; }

	requirement def FuelRequirement {
		subject vehicle : Vehicle;
		assume constraint nonnegativeFuel { vehicle.fuelMass >= 0 }
		require constraint { vehicle.fuelMass <= 2000 }
	}
	requirement fullTankRequirement : FuelRequirement {
		subject vehicle : Vehicle = vehicleUnderTest;
		assume constraint fullTank { vehicle.fuelMass == vehicle.fuelFullMass }
	}

	verification check {
		subject vehicle = vehicleUnderTest;
		objective { verify fullTankRequirement; }
		return verdict : VerdictKind =
			if allTrue(fullTankRequirement.assumptions()) ? PassIf(fullTankRequirement()) else VerdictKind::inconclusive;
	}
}
