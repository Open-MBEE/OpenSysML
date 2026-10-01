// A state's do behavior starts as the state is entered, and while a sibling
// region of the same entry still has an entry unit left, whether the do
// behavior's next move or that entry goes first is open: the do step is drawn
// against the sibling's entry as it is drawn against a dispatch.
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
				state r1 {
					entry { assign log := log + "r1(entry) "; }
				}
			}
		}
		transition first idle accept Go then work;
	}
	attribute def Go;
}
