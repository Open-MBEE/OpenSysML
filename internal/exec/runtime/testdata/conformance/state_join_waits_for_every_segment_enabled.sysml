// A join fires only when the occurrence enables every segment into it: one
// whose trigger the occurrence does not match, or whose guard is false, holds
// the join, and no segment's effect runs meanwhile.
package Test {
	private import ScalarValues::*;

	attribute def Go;
	attribute def Arm;

	state Machine {
		attribute log : String = "";
		attribute armed : Boolean = false;

		entry; then Work;
		state Work parallel {
			state a {
				entry; then a1;
				state a1;
				transition first a1 accept Go if armed do assign log := log + "a " then sync;
			}
			state b {
				entry; then b1;
				state b1;
				transition first b1 accept Go do assign log := log + "b " then sync;
			}
			state c {
				entry; then c1;
				state c1;
				transition first c1 accept Arm do assign armed := true then c1;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
	}
}
