// A subrequirement declaring a subject of its own, bound to nothing, is not
// bound to the container's subject (SysML v2 §7.21.2 binds only a subrequirement
// that declares none): the check reports its subject unbound instead of judging
// feather by it.
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def R {
		subject v : Veh;
		requirement child {
			subject s : Veh;
			require constraint { s.mass < 10.0 }
		}
	}
	part feather : Veh { attribute :>> mass = 1.0; }
	requirement r : R { subject v = feather; }
}
