// A body-local declaration in a constraint body answers to its declared type
// at run time (KerML 1.0 §7.3.4): Level's extent is {1, 3}, so holding 2 in a
// local typed by Level is a type mismatch, not a verdict.
package test {
	private import ScalarValues::*;

	enum def Level :> Integer { low = 1; high = 3; }

	constraint def Bounded {
		in n : Integer;
		attribute l : Level = n;
		l > 0
	}

	constraint refused : Bounded { in n = 2; }
}
