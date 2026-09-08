package test {
	private import ScalarValues::*;
	calc def Unary { in x : Real; return : Real; }
	calc def Fn { in calc f { in x : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc def Outer {
		in k : Real;
		calc inner { in x : Real; return : Real = x * k; }
		return : Real = Fn(inner, 2.0) + inner(3.0);
	}
	// A nested usage read bare computes over the enclosing run's bindings.
	calc def Bare {
		in k : Real;
		calc inner { in x : Real = 2.0; return : Real = x * k; }
		return : Real = inner;
	}
	// A nested calc returned as a function keeps the bindings of the run that returned it.
	calc def Maker {
		in k : Real;
		calc scale :> Unary { in :>> x; return : Real = x * k; }
		return : Unary = scale;
	}
	// A nested usage inheriting its body still closes over a default it declares in the run.
	calc def Mul { in a : Real; in b : Real; return : Real = a * b; }
	calc def Fixer { in k : Real; calc byK : Mul { in :>> b = k; } return : Mul = byK; }
	calc def ApplyOne { in calc f { in a : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc def UseOuter { return : Real = Outer(3.0) + Outer(4.0) + Bare(5.0) + Fn(Maker(10.0), 2.0) + ApplyOne(Fixer(10.0), 4.0); }
}
