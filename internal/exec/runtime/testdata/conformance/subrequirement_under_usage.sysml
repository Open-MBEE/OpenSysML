// A subrequirement nested in a requirement usage is one of that usage's required constraints (SysML v2 §7.21.2):
// LightReq holds for mass = 5.0, but `extra` nested in the usage fails, so r is not satisfied.
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def LightReq { subject s : Veh; require constraint { s.mass < 10.0 } }
	part veh : Veh { attribute :>> mass = 5.0; }
	requirement r : LightReq {
		subject s = veh;
		requirement extra { require constraint { s.mass > 30.0 } }
	}
}
