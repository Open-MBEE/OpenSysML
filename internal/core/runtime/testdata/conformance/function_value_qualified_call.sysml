package test {
	private import ScalarValues::*;
	calc def Sq { in v : Real; return : Real = v * v; }
	calc def Cube { in v : Real; return : Real = v * v * v; }
	// A calc-typed parameter called by its qualified name applies what the run bound it to.
	calc def Apply {
		in calc f { in v : Real; return : Real; }
		in a : Real;
		return : Real = Apply::f(a) + test::Apply::f(1.0);
	}
	// So does a name qualified by the calc the run specializes, and by the redeclaration.
	calc def Twice :> Apply {
		in calc g :>> f;
		return : Real = Apply::f(a) + Twice::g(a);
	}
	calc def UseQualified { return : Real = Apply(Sq, 3.0) + Twice(Cube, 2.0); }
}
