package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): one occurrence of Go
	// fires a transition in each region of the parallel state `work`; the
	// effects write different features neither region reads. The library orders
	// neither reaction before the other, and no order changes either value, so
	// the reactions commute and nothing is order-dependent.
	attribute def Go;
	state Machine {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		entry; then work;
		state work parallel {
			state a {
				entry; then a1;
				state a1;
				state a2;
				transition first a1 accept Go do assign x := 1 then a2;
			}
			state b {
				entry; then b1;
				state b1;
				state b2;
				transition first b1 accept Go do assign y := 2 then b2;
			}
		}
	}
}
