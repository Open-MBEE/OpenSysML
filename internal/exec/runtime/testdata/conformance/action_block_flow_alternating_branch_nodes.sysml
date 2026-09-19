// A loop that takes a different branch each iteration performs both branches'
// same-named nodes over time; each iteration's read of `p.v` is of the node
// its own branch declares, not of whichever `p` performed most recently. Odd
// iterations add i, even ones add 10*i: 1 + 20 + 3 + 40. Results report the
// latest performance under the shared name, the fourth iteration's 40.
package test {
	private import ScalarValues::*;

	action run {
		out total : Integer = 0;

		first start;
		then action iterate {
			for i in 1..4 {
				if i % 2 == 1 {
					action p {
						out v : Integer;
						assign v := i;
					}
					assign total := total + p.v;
				} else {
					action p {
						out v : Integer;
						assign v := i * 10;
					}
					assign total := total + p.v;
				}
			}
		}
		then done;
	}
}
