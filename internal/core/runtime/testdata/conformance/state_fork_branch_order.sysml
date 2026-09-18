// A fork's branches into the regions of one parallel state are drawn one unit
// at a time: each branch's effect, then the owner's entry — performed once, by
// whichever branch is drawn first — then the branch's target. The owner's
// entry follows the first branch effect and precedes both targets, so the four
// linearizations are the admissible set.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			entry { assign log := log + "work(entry) "; }
			state left {
				state a;
			}
			state right {
				state b;
			}
		}
		fork split;

		transition first idle accept Go then split;
		transition first split do { assign log := log + "T1.1(effect) "; } then a;
		transition first split do { assign log := log + "T1.2(effect) "; } then b;
	}
	attribute def Go;
}
