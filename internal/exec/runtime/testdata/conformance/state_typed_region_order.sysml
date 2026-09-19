// Entering and leaving a parallel state whose regions are typed usages: each
// region is entered at the initial state its definition declares, and leaving
// the composite exits every region in declaration order.
package test {
	state def Region {
		entry; then r1;
		state r1;
		state r2;

		transition first r1 when Step then r2;
	}

	state def Machine {
		entry; then start;
		state start;
		state running parallel {
			state a : Region;
			state b : Region;
		}
		state done;

		succession first start then running;
		transition first running when Abort then done;
	}
}
