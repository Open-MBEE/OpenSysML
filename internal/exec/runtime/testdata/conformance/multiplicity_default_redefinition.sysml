// A redefining feature is the redefined feature declared again, so a default it
// adds is bound by the multiplicity that declaration stated, and one it omits is
// the default the redefined declaration wrote.
package test {
	private import ScalarValues::Real;

	part def Base {
		attribute bare : Real[3];
		attribute valued : Real[2] = (1.0, 2.0);
	}

	part def Derived :> Base {
		// A default where the redefined feature had none.
		attribute :>> bare = (7.0, 8.0, 9.0);
		// No default, so the redefined declaration's stands, under both names.
		attribute kept :>> valued;
	}

	part derived : Derived;
}
