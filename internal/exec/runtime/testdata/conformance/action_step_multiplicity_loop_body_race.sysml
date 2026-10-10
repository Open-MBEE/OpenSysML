package test {
	private import ScalarValues::*;

	// A repeated step inside a stated loop body: each performance of `a`
	// reads `c` then writes back, so an interleaving between them admits
	// {c = 1, c = 2} — but the runtime performs each repetition as one move,
	// so exploration only observes {c = 2} and reports the miss as a note.
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