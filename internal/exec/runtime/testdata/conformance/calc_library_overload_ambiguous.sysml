// Two imported calcs the arguments fit equally well, neither more specific
// than the other: the call is reported ambiguous rather than dispatched.
package test {
	private import ScalarValues::*;

	package A {
		calc def pick { in x : Integer; in y : Real; return : Integer = 1; }
	}
	package B {
		calc def pick { in x : Real; in y : Integer; return : Integer = 2; }
	}

	part def Probe {
		private import A::*;
		private import B::*;
		attribute tied = pick(1, 2);
		attribute leftInteger = pick(1, 2.5);
		attribute rightInteger = pick(1.5, 2);
	}
}
