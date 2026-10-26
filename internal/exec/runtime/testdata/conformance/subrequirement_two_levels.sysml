// Subrequirements nest: `mid` takes R's subject, and `inner : LightReq` nested
// in mid takes mid's, so the leaf check reads veh two levels up (SysML v2 §7.21.2).
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def LightReq { subject s : Veh; require constraint { s.mass < 10.0 } }
	requirement def R {
		subject v : Veh;
		requirement mid { requirement inner : LightReq; }
	}
	part veh : Veh { attribute :>> mass = 20.0; }
	requirement r : R { subject v = veh; }
}
