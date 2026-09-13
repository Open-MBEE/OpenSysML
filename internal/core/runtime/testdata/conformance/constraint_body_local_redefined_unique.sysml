// A constraint local restating no multiplicity over the multi-valued unique
// declaration it redefines is judged for uniqueness as that declaration is
// (KerML 1.0 §7.3.4.5): two equal inherited values are refused.
package test {
	private import ScalarValues::*;

	constraint def Base {
		in n : Integer;
		attribute xs : Integer[*] = (n, n);
		SequenceFunctions::size(xs) > 0
	}

	constraint def Same :> Base {
		attribute redefines xs;
	}

	constraint refused : Same { in n = 1; }
}
