// A subrequirement's assumptions are its own check's (RequirementCheck::result =
// allTrue(assumptions()) implies allTrue(constraints())), not the container's: a
// false assumption makes the subrequirement hold vacuously and leaves the
// container's own required constraint to fail on veh.mass = 20.0.
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def R {
		subject v : Veh;
		require constraint { v.mass < 10.0 }
		requirement child {
			assume constraint { false }
			require constraint { false }
		}
	}
	part veh : Veh { attribute :>> mass = 20.0; }
	requirement r : R { subject v = veh; }
}
