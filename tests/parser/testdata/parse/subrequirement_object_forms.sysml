// The members of a requirement that are subrequirements (SysML v2 §7.21.2, §7.21.3): a typed, an untyped and a nested
// requirement usage, a plain and a framed concern — declared, typed or framed by reference — beside a referential one.
package test {
	part def Veh { attribute mass : ScalarValues::Real; }
	concern def Weight { subject v : Veh; require constraint { v.mass < 10.0 } }
	concern weight : Weight;
	requirement def LightReq { subject v : Veh; require constraint { v.mass < 10.0 } }
	requirement def R {
		subject v : Veh;
		requirement light : LightReq;
		requirement inl { require constraint { v.mass < 10.0 } }
		requirement deep { requirement deeper { require constraint { v.mass < 10.0 } } }
		requirement two[2] : LightReq;
		concern worry { require constraint { v.mass < 10.0 } }
		frame concern w : Weight;
		frame concern framed { require constraint { v.mass < 10.0 } }
		frame weight;
		ref requirement refd : LightReq;
	}
}
