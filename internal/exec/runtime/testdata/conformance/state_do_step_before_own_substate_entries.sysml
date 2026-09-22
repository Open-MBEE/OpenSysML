// A composite's own do behavior begins at its entry: its step is drawn against
// the entries of its own orthogonal substates on the front entering them, so it
// falls before, between or after them in either of their orders.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			do { assign log := log + "did "; }
			state left {
				entry; then l1;
				state l1 {
					entry { assign log := log + "l1(entry) "; }
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
