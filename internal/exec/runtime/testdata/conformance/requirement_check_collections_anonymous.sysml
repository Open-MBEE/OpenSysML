// An anonymous `assume constraint { … }` or `require constraint { … }` is a member of
// `Requirements::RequirementCheck::assumptions` or `constraints` exactly as a named one is
// (SysML v2 §8.3.20.4 checkConstraintUsageRequirementConstraintSpecialization): the
// collections hold the requirement usage's own anonymous constraints and the ones it
// inherits from its definition, so `allTrue(constraints())` reads the violated inherited
// `fuelMass <= 2000` rather than a vacuous empty sequence.
package test {
	private import ScalarValues::Integer;
	private import ScalarValues::Boolean;
	private import ControlFunctions::allTrue;
	private import VerificationCases::VerdictKind;

	part def Vehicle { attribute fuelMass : Integer; attribute fuelFullMass : Integer; }
	part vehicleUnderTest : Vehicle { attribute redefines fuelMass = 2500; attribute redefines fuelFullMass = 2500; }

	requirement def FuelRequirement {
		subject vehicle : Vehicle;
		assume constraint { vehicle.fuelMass >= 0 }
		require constraint { vehicle.fuelMass <= 2000 }
	}
	part sys {
		requirement fullTankRequirement : FuelRequirement {
			subject vehicle : Vehicle = vehicleUnderTest;
			assume constraint { vehicle.fuelMass == vehicle.fuelFullMass }
			require constraint { vehicle.fuelMass > 0 }
		}
	}

	attribute assumptionResults : Boolean[0..*] nonunique = sys.fullTankRequirement.assumptions.result; // the usage's own assumption, then the inherited one
	attribute assumptionsAllTrue : Boolean = allTrue(sys.fullTankRequirement.assumptions());
	attribute constraintResults : Boolean[0..*] = sys.fullTankRequirement.constraints.result; // the usage's own constraint, then the inherited one
	attribute constraintsAllTrue : Boolean = allTrue(sys.fullTankRequirement.constraints());
	attribute requirementResult : Boolean = sys.fullTankRequirement.result;

	verification check {
		subject vehicle = vehicleUnderTest;
		objective { verify sys.fullTankRequirement; }
		return verdict : VerdictKind =
			if allTrue(sys.fullTankRequirement.constraints()) ? VerdictKind::pass else VerdictKind::fail;
	}
}
