// The sibling region's state completes into a transition that leaves the
// parallel state: the do behavior's two steps are drawn against the sibling's
// entry while it is left, then against the completion's dispatch, which exits
// the do behavior's state and cuts it off wherever it stands.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				entry; then l1;
				state l1 {
					do { assign log := log + "a "; assign log := log + "b "; }
				}
			}
			state right {
				entry; then r1;
				state r1 {
					entry { assign log := log + "r1(entry) "; }
				}
				transition first r1 then stopped;
			}
		}
		state stopped;
		transition first idle accept Go then work;
	}
	attribute def Go;
}
