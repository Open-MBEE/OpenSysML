// A requirement's assume and require constraints are the objects its
// `assumptions` and `constraints` collections hold (Requirements.sysml
// `RequirementCheck`), each a constraint check whose `result` is the usage's
// condition evaluated on the requirement, inherited ones included; invoking the
// collection (`assumptions()`) is each member's result (Requirements.sysml
// `allTrue(assumptions()) implies allTrue(constraints())`), from the
// requirement's own context, through a chain and from a verification case (#1854).
// The inherited assumption (the definition's `nonnegativeFuel`) is false: the
// requirement is vacuously true, its assumptions are not all true, and the
// verification is inconclusive.
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
		assume constraint nonnegativeFuel :> assumptions { vehicle.fuelMass >= 0 }
		require constraint fuelLimit :> constraints { vehicle.fuelMass <= 2000 }
	}
	part sys {
		requirement fullTankRequirement : FuelRequirement {
			subject vehicle : Vehicle = vehicleUnderTest;
			assume constraint fullTank :> assumptions { vehicle.fuelMass == vehicle.fuelFullMass }
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
