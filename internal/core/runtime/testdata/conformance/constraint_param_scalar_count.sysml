// A constraint parameter stating no multiplicity holds one value (KerML 1.0
// §7.3.4.5): two values bound to it at run time are a multiplicity violation,
// not a collection the condition may measure.
package test {
	private import ScalarValues::*;

	attribute vals : Integer[*] = (1, 2);

	constraint def Scalar {
		in x : Integer;
		SequenceFunctions::size(x) == 2
	}

	constraint refused : Scalar { in x = vals; }
}
