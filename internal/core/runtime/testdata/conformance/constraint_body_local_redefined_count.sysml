// A constraint local inheriting its initializer from the declaration it
// redefines answers to the redefinition's multiplicity (KerML 1.0 §7.3.4.5):
// three inherited values bound to a local redeclared `[2]` are refused.
package test {
	private import ScalarValues::*;

	constraint def Base {
		in n : Integer;
		attribute xs : Integer[*] = (n, n + 1, n + 2);
		SequenceFunctions::size(xs) > 0
	}

	constraint def Tight :> Base {
		attribute redefines xs : Integer[2];
	}

	constraint refused : Tight { in n = 1; }
}
