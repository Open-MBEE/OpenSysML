// The reporter's own spelling: `assume constraint` and `require constraint` with no
// explicit `:> assumptions` / `:> constraints` subset the collections implicitly
// (SysML v2 §8.3.20.4), so the members, their `result`, the collection's invocation
// and the verification read as with the explicit form; `(-200, -200)`: the inherited
// assumption is false, the requirement vacuously true, the verification inconclusive (#1854).
// The required constraint is named: an anonymous `require constraint { … }` is no feature
// of the object's shape yet, so it counts in the requirement's own result but not in `constraints`.
package test {
	private import ScalarValues::Integer;
	private import ScalarValues::Boolean;
	private import ControlFunctions::allTrue;
	private import VerificationCases::VerdictKind;
	private import VerificationCases::PassIf;

	part def Vehicle { attribute fuelMass : Integer; attribute fuelFullMass : Integer; }
	part vehicleUnderTest : Vehicle { attribute redefines fuelMass = -200; attribute redefines fuelFullMass = -200; }

	requirement def FuelRequirement {
		subject vehicle : Vehicle;
		assume constraint nonnegativeFuel { vehicle.fuelMass >= 0 }
		require constraint fuelLimit { vehicle.fuelMass <= 2000 }
	}
	part sys {
		requirement fullTankRequirement : FuelRequirement {
			subject vehicle : Vehicle = vehicleUnderTest;
			assume constraint fullTank { vehicle.fuelMass == vehicle.fuelFullMass }
		}
	}

	attribute assumptionResults : Boolean[0..*] = sys.fullTankRequirement.assumptions.result; // the usage's own assumption, then the inherited one
	attribute assumptionsAllTrue : Boolean = allTrue(sys.fullTankRequirement.assumptions());
	attribute assumptionsAllTrueRead : Boolean = allTrue(sys.fullTankRequirement.assumptions.result);
	attribute constraintResults : Boolean[0..*] = sys.fullTankRequirement.constraints.result;
	attribute constraintsAllTrue : Boolean = allTrue(sys.fullTankRequirement.constraints());
	attribute inheritedAssumption : Boolean = sys.fullTankRequirement.nonnegativeFuel.result;
	attribute localAssumption : Boolean = sys.fullTankRequirement.fullTank.result;
	attribute requirementResult : Boolean = sys.fullTankRequirement.result;

	verification check {
		subject vehicle = vehicleUnderTest;
		objective { verify sys.fullTankRequirement; }
		return verdict : VerdictKind =
			if allTrue(sys.fullTankRequirement.assumptions()) ? PassIf(sys.fullTankRequirement()) else VerdictKind::inconclusive;
	}
}
