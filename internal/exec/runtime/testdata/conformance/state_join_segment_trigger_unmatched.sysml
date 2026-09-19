// A segment into a join whose trigger the occurrence does not match holds the
// join: the other segment, though enabled, fires nothing and runs no effect.
package Test {
	private import ScalarValues::*;

	attribute def Go;
	attribute def Nudge;

	state Machine {
		attribute log : String = "";

		entry; then Work;
		state Work parallel {
			state a {
				entry; then a1;
				state a1;
				transition first a1 accept Go do assign log := log + "a " then sync;
			}
			state b {
				entry; then b1;
				state b1;
				transition first b1 accept Nudge do assign log := log + "b " then sync;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
	}
}
