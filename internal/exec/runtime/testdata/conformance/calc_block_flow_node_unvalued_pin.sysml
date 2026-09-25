// An action node in a calc's loop body holds the pins it declares without a value
// as its own: `v` is assigned in the node's frame rather than in the calc's
// same-named local, and `w`, which the calc does not declare at all, is the
// node's to assign. The result is 163 only if the calc's `v` stayed 100.
package test {
	private import ScalarValues::*;

	calc noted {
		attribute v : Integer = 100;
		attribute total : Integer = 0;
		for i in 1..3 {
			action note {
				out v : Integer;
				out w : Integer;
				assign v := i * 10;
				assign w := v + 1;
				assign total := total + w;
			}
		}
		return : Integer = v + total;
	}
}
