// A subrequirement declaring its own subject keeps it (SysML v2 §7.21.2): `pow` is about the vehicle's engine, not
// the vehicle, and holds for power = 200.0 although the vehicle itself would fail a mass check.
package test {
	private import ScalarValues::*;
	part def Eng { attribute power : Real; }
	part def Veh { attribute mass : Real; part eng : Eng; }
	requirement def PowerReq { subject e : Eng; require constraint { e.power > 100.0 } }
	requirement def R {
		subject v : Veh;
		requirement pow : PowerReq { subject e = v.eng; }
	}
	part heavy : Veh { attribute :>> mass = 20.0; part :>> eng { attribute :>> power = 200.0; } }
	requirement r : R { subject v = heavy; }
}
