// An argument of statically unknown type keeps every overload applicable: the run
// settles the call by the values' types as the checker would have by the declared
// ones, and reports a tie they leave.
package test {
	private import ScalarValues::*;

	part def A {
		calc def label { in xs : Integer[*]; return : String = "integer"; }
	}
	part def B {
		calc def label { in xs : String[*]; return : String = "string"; }
	}
	part def C {
		calc def pick { in x : Integer; in y : Real; return : Integer = 1; }
	}
	part def D {
		calc def pick { in x : Real; in y : Integer; return : Integer = 2; }
	}

	part def Probe :> A, B, C, D {
		calc def labelOf { in v[*]; return : String = label(v); }
		calc def pickOf { in v; in w; return : Integer = pick(v, w); }
		attribute fromInteger = labelOf(3);
		attribute fromString = labelOf("s");
		attribute fromSequence = labelOf((1, 2));
		attribute fromMixed = labelOf((1, "s"));
		attribute settled = pickOf(1, 2.5);
		attribute stillTied = pickOf(1, 2);
	}
}
