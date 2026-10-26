// A typed subrequirement declaring no subject takes the container's: LightReq's
// subject s is bound to R's v, so `light : LightReq` reads veh (SysML v2 §7.21.2).
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def LightReq { subject s : Veh; require constraint { s.mass < 10.0 } }
	requirement def R {
		subject v : Veh;
		requirement light : LightReq;
	}
	part veh : Veh { attribute :>> mass = 5.0; }
	requirement r : R { subject v = veh; }
}
