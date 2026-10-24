package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): one occurrence of Go
	// fires a transition in each region of the parallel state `work`, and both
	// effects write `last`. The library orders neither region's reaction before
	// the other's, so the last write stands: `last` is order-dependent, and a
	// default run reports it only up to that order.
	attribute def Go;
	state Machine {
		attribute last : Integer = 0;

		entry; then work;
		state work parallel {
			state a {
				entry; then a1;
				state a1;
				state a2;
				transition first a1 accept Go do assign last := 1 then a2;
			}
			state b {
				entry; then b1;
				state b1;
				state b2;
				transition first b1 accept Go do assign last := 2 then b2;
			}
		}
	}
}
