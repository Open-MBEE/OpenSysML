// The read rule for a bare calc-usage name is by what the usage declares, not by
// invocation: an input with no written multiplicity is unsupplied, so `fBare`
// reads as the function `Fn` applies, while an input written `[0..1]` is
// supplied, so `fOpt` reads as its computed value.
package test {
	private import ScalarValues::*;
	calc def Fn { in calc f { in x : Real; return : Real; } in a : Real; return : Real = f(a); }
	calc useShapes {
		in k : Real = 3.0;
		calc fBare { in x : Real; return : Real = x * k; }
		calc fOpt { in x : Real[0..1]; return : Real = x ?? k; }
		return : Real = Fn(fBare, 2.0) + fOpt;
	}
}
