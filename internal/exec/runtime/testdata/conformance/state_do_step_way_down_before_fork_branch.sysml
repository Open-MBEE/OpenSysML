// A fork's branches enter a composite on their way down: the composite's own do
// behavior begins when the first branch enters it and its step is drawn against
// the branches' remaining effect and target entries.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			do { assign log := log + "did "; }
			state left {
				state a {
					entry { assign log := log + "a(entry) "; }
				}
			}
			state right {
				state b {
					entry { assign log := log + "b(entry) "; }
				}
			}
		}
		fork split;

		transition first idle accept Go then split;
		transition first split then a;
		transition first split then b;
	}
	attribute def Go;
}
