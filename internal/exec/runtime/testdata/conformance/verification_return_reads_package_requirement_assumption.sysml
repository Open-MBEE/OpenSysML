// A verification's returned expression reads a package-level requirement's assumed
// constraints as the verdicts their checks reach on the requirement's bound subject —
// `false` for the violated one and `true` for the one that holds, never undetermined —
// whether read through `result` or called on the requirement.
package test {
	private import ScalarValues::Integer;
	private import VerificationCases::VerdictKind;

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
			if fullTankRequirement.nonnegativeFuel.result == false
				and fullTankRequirement.nonnegativeFuel() == false
				and fullTankRequirement.fullTank.result
				and fullTankRequirement.fullTank() ? VerdictKind::pass else VerdictKind::fail;
	}
}
