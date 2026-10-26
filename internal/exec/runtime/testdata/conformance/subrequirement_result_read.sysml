// The container's `result` (Requirements.sysml `RequirementCheck::result =
// allTrue(assumptions()) implies allTrue(constraints())`) counts a subrequirement
// among the constraints: rr.result is false for veh.mass = 20.0, rrOk.result true.
package test {
	private import ScalarValues::*;
	part def Veh { attribute mass : Real; }
	requirement def R {
		subject v : Veh;
		requirement light { require constraint { v.mass < 10.0 } }
	}
	part veh : Veh { attribute :>> mass = 20.0; }
	part feather : Veh { attribute :>> mass = 5.0; }
	part box {
		requirement rr : R { subject v = veh; }
		requirement rrOk : R { subject v = feather; }
	}
	attribute rrResult = box.rr.result;
	attribute rrOkResult = box.rrOk.result;
	calc probe { return one = 1; }
}
