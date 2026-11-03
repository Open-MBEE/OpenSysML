// Inside a subrequirement an `assume constraint` stays an assumption and a `require constraint` a required constraint of
// the container: the assumption holds on veh (mass 20.0 > 1.0), so the check fails on the required one.
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def R {
		subject v : Veh;
		requirement guarded {
			assume constraint { v.mass > 1.0 }
			require constraint { v.mass < 10.0 }
		}
	}
	part veh : Veh { attribute :>> mass = 20.0; }
	requirement r : R { subject v = veh; }
}
