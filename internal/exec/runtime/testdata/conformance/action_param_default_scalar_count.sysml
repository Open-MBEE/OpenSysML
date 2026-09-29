// An action parameter declaring [1] rejects a default that yields two values
// when the node's own features are seeded, as an argument of two would be.
package test {
	private import ScalarValues::*;

	action run {
		attribute src : Integer[*] = (1, 2);
		out r : Integer = 0;

		first start;

		action hold {
			in x : Integer[1] = src;
			assign r := SequenceFunctions::size(x);
		}

		done;

		succession first start then hold;
		succession first hold then done;
	}
}
