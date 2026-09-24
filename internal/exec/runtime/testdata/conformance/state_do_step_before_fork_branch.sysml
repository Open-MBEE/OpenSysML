// A fork's branches enter two regions: the do behavior of the target one
// branch reaches is drawn against the other branch's effect and target entry
// while either is left, as a region's do step is drawn against a sibling's.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				state a {
					do { assign log := log + "did "; }
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
		transition first split do { assign log := log + "T1.2(effect) "; } then b;
	}
	attribute def Go;
}
