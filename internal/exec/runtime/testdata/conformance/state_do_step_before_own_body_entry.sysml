// A composite's own do behavior against the entries of its serial body, which no
// front orders: the way down from the composite to its start state's own start
// is drawn against the step, each entry a unit, so the step falls before,
// between or after the two entries.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work {
			do { assign log := log + "did "; }
			entry; then w1;
			state w1 {
				entry action { assign log := log + "w1(entry) "; } then w2;
				state w2 {
					entry { assign log := log + "w2(entry) "; }
				}
			}
		}
		transition first idle accept Go then work;
	}
	attribute def Go;
}
