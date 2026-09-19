// A timer's expiry enables one segment into a join but not the other, which
// waits for a signal: the join does not fire, and the route out of it is not
// resolved, so the junction beyond it, whose only guard is false, is not read
// and the expiry aborts nothing. The signal holds the join the same way.
package Test {
	private import ScalarValues::*;
	private import SI::*;

	attribute def Go;

	state Machine {
		attribute log : String = "";
		attribute ready : Boolean = false;

		entry; then Work;
		state Work parallel {
			state a {
				entry; then a1;
				state a1;
				transition first a1 accept after 1 [s] do assign log := log + "a " then sync;
			}
			state b {
				entry; then b1;
				state b1;
				transition first b1 accept Go do assign log := log + "b " then sync;
			}
		}
		join sync;
		junction pick;
		transition first sync do assign log := log + "sync " then pick;
		transition first pick if ready then done;
		state done;
	}
}
