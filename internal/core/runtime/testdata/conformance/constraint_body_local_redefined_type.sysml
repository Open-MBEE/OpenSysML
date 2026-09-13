// A constraint local inheriting its initializer from the declaration it
// redefines answers to the redefinition's type (KerML 1.0 §7.3.4.5): the
// inherited `n` is judged as a Level, not as the Integer the base declared.
package test {
	private import ScalarValues::*;

	enum def Level :> Integer { low = 1; high = 3; }

	constraint def Base {
		in n : Integer;
		attribute l : Integer = n;
		l > 0
	}

	constraint def Tight :> Base {
		attribute redefines l : Level;
	}

	constraint refused : Tight { in n = 2; }
}
