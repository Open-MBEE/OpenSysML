// A fork enters two regions at states that complete at once, silently: neither
// branch has an effect and neither target performs anything, yet each target's
// entry generates a completion event, so the fork's entry draw decides the
// pool's order and the completions dispatch in the order the entries were
// performed. The effects log that order; both orders are admissible.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				state a;
				state a2;
				transition first a do assign log := log + "a " then a2;
			}
			state right {
				state b;
				state b2;
				transition first b do assign log := log + "b " then b2;
			}
		}
		fork split;

		transition first idle accept Go then split;
		transition first split then a;
		transition first split then b;
	}
	attribute def Go;
}
