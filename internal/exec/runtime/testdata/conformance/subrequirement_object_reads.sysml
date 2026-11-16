// A part's object holds one object per subrequirement of `rr`, typed, untyped or unnamed, in `subrequirements` and `constraints`; each
// reads the container's subject unless it declares one; one of optional multiplicity is not held (SysML v2 §7.21.2, §8.4.17.1).
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::size;
	part def Veh { attribute mass : Real; part eng : Eng; }
	part def Eng { attribute power : Real; }
	requirement def LightReq { subject v : Veh; require constraint { v.mass < 10.0 } }
	requirement def PowerReq { subject e : Eng; require constraint { e.power > 1.0 } }
	requirement def R {
		subject v : Veh;
		requirement light : LightReq;
		requirement inl { require constraint { v.mass < 10.0 } }
		requirement pow : PowerReq { subject e = v.eng; }
		requirement deep { requirement deeper { require constraint { v.mass < 10.0 } } }
		requirement guarded { assume constraint { v.mass < 1.0 } require constraint { v.mass < 0.0 } }
		concern worry { require constraint { v.mass < 10.0 } }
		ref requirement refd : LightReq;
		assume constraint { v.mass > 0.0 }
		require constraint named { v.mass > 1.0 }
	}
	requirement def Unnamed {
		subject v : Veh;
		requirement { require constraint { v.mass < 10.0 } }
		requirement : LightReq;
		requirement { require constraint { v.mass < 30.0 } }
	}
	requirement def Optional { subject v : Veh; requirement opt[0..1] : LightReq; requirement many[*] : LightReq; }
	requirement def Owned {
		subject v : Veh;
		requirement own { subject s : Veh; require constraint { s.mass < 10.0 } }
	}
	part veh : Veh { attribute :>> mass = 20.0; part :>> eng { attribute :>> power = 5.0; } }
	part feather : Veh { attribute :>> mass = 5.0; part :>> eng { attribute :>> power = 0.5; } }
	part box {
		requirement rr : R { subject v = veh; }
		requirement rrOk : R { subject v = feather; }
		requirement bare : R;
		requirement owned : Owned { subject v = feather; }
		requirement unnamed : Unnamed { subject v = veh; }
		requirement optional : Optional { subject v = veh; }
		satisfy requirement sat : R by feather;
	}
	part holder : Veh { attribute :>> mass = 3.0; satisfy requirement impl : R; }

	attribute rrResult = box.rr.result;
	attribute rrLight = box.rr.light.result;
	attribute rrInl = box.rr.inl.result;
	attribute rrPow = box.rr.pow.result;
	attribute rrDeep = box.rr.deep.result;
	attribute rrDeeper = box.rr.deep.deeper.result;
	attribute rrGuarded = box.rr.guarded.result;
	attribute rrWorry = box.rr.worry.result;
	attribute rrSubrequirements = size(box.rr.subrequirements);
	attribute rrConstraints = size(box.rr.constraints);
	attribute rrAssumptions = size(box.rr.assumptions);
	attribute rrConcerns = size(box.rr.concerns);
	attribute rrDeepSubrequirements = size(box.rr.deep.subrequirements);
	attribute rrSubResults : Boolean[*] nonunique = box.rr.subrequirements.result;
	attribute rrLightMass = box.rr.light.v.mass;
	attribute rrInlSubject = box.rr.inl.subj == veh;
	attribute rrDeeperSubject = box.rr.deep.deeper.subj == veh;
	attribute rrPowPower = box.rr.pow.e.power;

	attribute okResult = box.rrOk.result;
	attribute okLight = box.rrOk.light.result;
	attribute okInl = box.rrOk.inl.result;
	attribute okPow = box.rrOk.pow.result;
	attribute okDeeper = box.rrOk.deep.deeper.result;
	attribute okLightMass = box.rrOk.light.v.mass;

	attribute bareLight = box.bare.light.result;
	attribute bareInlSubject = box.bare.inl.subj;
	attribute bareSubrequirements = size(box.bare.subrequirements);
	attribute ownedOwn = box.owned.own.result;
	attribute ownedOwnSubject = box.owned.own.s;

	attribute unnamedResult = box.unnamed.result;
	attribute unnamedSubrequirements = size(box.unnamed.subrequirements);
	attribute unnamedSubResults : Boolean[*] nonunique = box.unnamed.subrequirements.result;

	attribute optionalResult = box.optional.result;
	attribute optionalSubrequirements = size(box.optional.subrequirements);

	attribute satLight = box.sat.light.result;
	attribute satLightMass = box.sat.light.v.mass;
	attribute implLight = holder.impl.light.result;
	attribute implInlMass = holder.impl.light.v.mass;
	calc probe { return one = 1; }
}
