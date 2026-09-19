// A completion segment into a join is enabled once its source has completed,
// its do behavior finished included: a Go arriving while b1's body still waits
// for a Tick does not fire the join, and neither does b1's completion alone; the
// Go that comes once b1 has completed does.
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
