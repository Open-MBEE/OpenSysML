// A composite requirement usage nested in a requirement definition is a subrequirement: a required constraint of its container, and declaring no subject of its own it is about the
// container's subject (SysML v2 §7.21.2, Requirements.sysml `subrequirements :> requirementChecks, constraints`). `light` fails for veh.mass = 20.0, so r is not satisfied.
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def R {
		subject v : Veh;
		requirement light { require constraint { v.mass < 10.0 } }
	}
	part veh : Veh { attribute :>> mass = 20.0; }
	requirement r : R { subject v = veh; }
}
