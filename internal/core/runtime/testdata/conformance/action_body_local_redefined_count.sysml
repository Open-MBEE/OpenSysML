// An action local restating no multiplicity over the declaration it redefines
// inherits that declaration's bound (KerML 1.0 §7.3.4.5): three values seeded
// into a local redefining a `[2]` are a multiplicity violation.
package test {
	private import ScalarValues::*;

	action def Two {
		attribute xs : Integer[2];
	}

	action run {
		out r : Integer = 0;

		first start;

		action hold : Two {
			attribute redefines xs = (1, 2, 3);
			assign r := SequenceFunctions::size(xs);
		}

		done;

		succession first start then hold;
		succession first hold then done;
	}
}
