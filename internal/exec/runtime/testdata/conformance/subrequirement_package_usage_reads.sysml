// A package-level requirement usage is read through an object made and taken back with the read: a subrequirement's `result` and
// subject are the container's (SysML v2 §7.21.2); `subrequirements` and `constraints` stay unread, but their `size` is determined.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::size;
	part def Veh { attribute mass : Real; }
	requirement def LightReq { subject v : Veh; require constraint { v.mass < 10.0 } }
	requirement def R {
		subject v : Veh;
		requirement light : LightReq;
		requirement inl { require constraint { v.mass < 10.0 } }
		requirement two[2] : LightReq;
		requirement own { subject s : Veh; require constraint { s.mass < 10.0 } }
		require constraint { v.mass > 1.0 }
	}
	part veh : Veh { attribute :>> mass = 20.0; }
	part feather : Veh { attribute :>> mass = 5.0; }
	requirement top : R { subject v = veh; }
	requirement topOk : R { subject v = feather; }
	requirement unbound : R;

	attribute topLight = top.light.result;
	attribute topInl = top.inl.result;
	attribute topTwo : Boolean[*] nonunique = top.two.result;
	attribute topLightMass = top.light.v.mass;
	attribute topOwn = top.own.result;
	attribute topSubrequirements = size(top.subrequirements);
	attribute topConstraints = size(top.constraints);
	ref requirement topCollection[*] = top.subrequirements;
	attribute okLight = topOk.light.result;
	attribute okInl = topOk.inl.result;
	attribute okLightMass = topOk.light.v.mass;
	attribute unboundLight = unbound.light.result;
	attribute unboundSubrequirements = size(unbound.subrequirements);
	calc probe { return one = 1; }
}
