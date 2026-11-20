// The slots of an instantiated object holding requirement usages: each holds an object whose subrequirements are objects of
// their own (SysML v2 §7.21.2), reading their container's subject and answering their own check as `result`.
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def LightReq { subject v : Veh; require constraint { v.mass < 10.0 } }
	requirement def R {
		subject v : Veh;
		requirement light : LightReq;
		requirement inl { require constraint { v.mass < 10.0 } }
		requirement deep { requirement deeper { require constraint { v.mass < 10.0 } } }
		frame concern worry { require constraint { v.mass < 10.0 } }
	}
	part def Box {
		part heavy : Veh { attribute :>> mass = 20.0; }
		part feather : Veh { attribute :>> mass = 5.0; }
		requirement rr : R { subject v = heavy; }
		requirement rrOk : R { subject v = feather; }
		requirement bare : R;
	}
}
