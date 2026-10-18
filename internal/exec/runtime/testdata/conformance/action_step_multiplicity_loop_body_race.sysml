package test {
	private import ScalarValues::*;

	// A repeated step inside a stated loop body: each performance of `a`
	// reads `c` then writes back, so the two interleave as two top-level
	// performances do and admit {c = 1, c = 2}; the declared schedule finishes
	// one before the other starts.
	action def U {
		attribute c : Integer = 0;
		first start then b;
		action b {
			attribute i : Integer = 0;
			while i < 1 {
				first start then a;
				action a[2] {
					attribute t : Integer := c;
					assign c := t + 1;
				}
				succession first [*] a then [1] tally;
				action tally {
					assign i := i + 1;
				}
			}
		}
		then done;
	}
}