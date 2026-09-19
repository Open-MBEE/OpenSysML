// A region whose start state is itself parallel appends that state's regions to
// the front being drawn: its two regions' entries interleave with the sibling
// region's entry as three queues of one front, not as one unit. The six
// linearizations are the admissible set.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				entry; then inner;
				state inner parallel {
					state p {
						entry { assign log := log + "p(entry) "; }
						entry; then p1;
						state p1;
					}
					state q {
						entry { assign log := log + "q(entry) "; }
						entry; then q1;
						state q1;
					}
				}
			}
			state right {
				entry { assign log := log + "right(entry) "; }
				entry; then r;
				state r;
			}
		}

		transition first idle accept Go then work;
	}
	attribute def Go;
}
