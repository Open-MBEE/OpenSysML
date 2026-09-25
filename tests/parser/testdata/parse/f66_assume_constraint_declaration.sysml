// F66: `AssumeConstraintUsage` is `'assume' ConstraintUsageKeyword
// ConstraintUsageDeclaration CalculationBody` (SysML.xtext:2070), so the owned
// constraint may be named and typed and may end with ';' instead of a body.
package AssumedConstraints {
	private import ScalarValues::*;
	constraint def C;
	part def Vehicle { attribute fuelLevel : Real; }
	requirement def R {
		subject vehicle : Vehicle;
		assume constraint c1 : C;
		assume constraint fuelConstraint { vehicle.fuelLevel >= 0.0 }
		require constraint r1 : C;
		require constraint { vehicle.fuelLevel < 100.0 }
	}
}
