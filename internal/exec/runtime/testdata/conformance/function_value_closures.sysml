package test {
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Half { in v : Real; return : Real = v / 2.0; }
	calc sqU : Sq;
	calc halfU : Half;
	calc def Apply { in calc f { in v : Real; return : Real; } in a : Real; return : Real = f(a); }
	// A function chosen at run time, returned, stored and reassigned.
	calc def Pick { in b : Boolean; return r = if b ? sqU else halfU; }
	calc def Stored { in b : Boolean; in a : Real; attribute g = sqU; if b { assign g := halfU; } return : Real = Apply(g, a) + Apply(g, 1.0); }
	// A body-local calc returned keeps the run that read it; two runs give two functions.
	calc def Mk { in k : Real; calc s { in v : Real; return : Real = v * k; } return r = s; }
	// A closure calling itself, and one returning a sibling, read the same run.
	calc def Rec { in k : Real; calc s { in v : Real; return : Real = if v > 3.0 ? v else s(v + k); } return r = s(1.0); }
	calc def Sib { in k : Real; calc u { in v : Real; return : Real = v + k; } calc s { in v : Real; return r = u; } return r = Apply(s(1.0), 2.0) + (if s(1.0) == u ? 1000.0 else 0.0); }
	calc def UseAll {
		return : Real = Apply(Pick(true), 3.0) + Apply(Pick(false), 3.0) + Stored(true, 4.0) + Stored(false, 4.0)
			+ Rec(1.0) + Sib(1.0) + Apply(Mk(2.0), 5.0)
			+ (if Mk(2.0) == Mk(2.0) ? 10000.0 else 0.0) + (if Pick(true) == sqU ? 100.0 else 0.0);
	}
}
