// A do step against several remaining sibling entries: the do behavior of the
// first region's state is drawn against each of the two other regions' entries
// while one is left, so it falls before, between or after them in either of
// their orders.
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
			state middle {
				entry; then m1;
				state m1 {
					entry { assign log := log + "m1(entry) "; }
				}
			}
			state right {
				entry; then r1;
				state r1 {
					entry { assign log := log + "r1(entry) "; }
				}
			}
		}
		transition first idle accept Go then work;
	}
	attribute def Go;
}
