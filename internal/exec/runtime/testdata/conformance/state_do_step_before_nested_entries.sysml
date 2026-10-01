// A do step against a sibling region whose start state is itself parallel: the
// nested regions' entries join the front the do step is drawn from, so the step
// falls before, between or after them in either of their orders.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				entry; then l1;
				state l1 {
					do { assign log := log + "did "; }
				}
			}
			state right {
				entry; then r1;
				state r1 parallel {
					state ra {
						entry; then a1;
						state a1 {
							entry { assign log := log + "a1(entry) "; }
						}
					}
					state rb {
						entry; then b1;
						state b1 {
							entry { assign log := log + "b1(entry) "; }
						}
					}
				}
			}
		}
		transition first idle accept Go then work;
	}
	attribute def Go;
}
