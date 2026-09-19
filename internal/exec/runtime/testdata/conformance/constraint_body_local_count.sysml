// A body-local declaration in a constraint body answers to its stated
// multiplicity at run time (KerML 1.0 §7.3.4): three values bound to a local
// declared to hold two are a multiplicity violation, not a verdict.
package test {
	private import ScalarValues::*;

	constraint def TwoOf {
		in n : Integer;
		attribute xs : Integer[2] = (n, n + 1, n + 2);
		SequenceFunctions::size(xs) > 0
	}

	constraint refused : TwoOf { in n = 1; }
}
