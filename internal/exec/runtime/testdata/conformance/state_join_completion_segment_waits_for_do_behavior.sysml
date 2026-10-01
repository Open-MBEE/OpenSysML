// A segment fires on its own occurrence and waits: Go records a's arrival while
// b's do behavior waits, then b's completion finishes the join after Tick.
package Test {
	private import ScalarValues::*;

	attribute def Go;
	attribute def Tick;

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
				state b1 {
					do action busy {
						first start;
						then action wait accept Tick;
						then action mark assign log := log + "tick ";
						then done;
					}
				}
				transition first b1 do assign log := log + "b " then sync;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
	}
}
