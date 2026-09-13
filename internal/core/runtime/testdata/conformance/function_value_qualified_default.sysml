package test {
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc twice { in v : Real; return : Real = v * 2.0; }
	// A calc-typed parameter whose value names a calc applies that calc when called by
	// its qualified name outside any run of its owner, as the bare name does; a run
	// that bound it applies the binding instead.
	calc def Apply {
		in calc f = twice;
		in a : Real;
		return : Real = Apply::f(a) + f(a);
	}
	calc def UseDefault { return : Real = Apply::f(3.0) + Apply(Sq, 3.0) + Apply(a = 4.0); }
}
