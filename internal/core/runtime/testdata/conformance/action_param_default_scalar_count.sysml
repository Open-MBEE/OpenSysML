// An action parameter stating no multiplicity holds one value (KerML 1.0
// §7.4.5): a default that yields two at run time is a multiplicity violation
// when the node's own features are seeded, as an argument of two would be.
package test {
	private import ScalarValues::*;

	action run {
		attribute src : Integer[*] = (1, 2);
		out r : Integer = 0;

		first start;

		action hold {
			in x : Integer = src;
			assign r := SequenceFunctions::size(x);
		}

		done;

		succession first start then hold;
		succession first hold then done;
	}
}
