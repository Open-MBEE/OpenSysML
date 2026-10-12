// Two inherited calcs the arguments fit equally well, neither more specific
// than the other: the call is reported ambiguous rather than dispatched.
package test {
	private import ScalarValues::*;

	part def A {
		calc def pick { in x : Integer; in y : Real; return : Integer = 1; }
	}
	part def B {
		calc def pick { in x : Real; in y : Integer; return : Integer = 2; }
	}

	part def Probe :> A, B {
		attribute tied = pick(1, 2);
		attribute leftInteger = pick(1, 2.5);
		attribute rightInteger = pick(1.5, 2);
	}
}
