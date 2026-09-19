// A join fires on one occurrence enabling every segment into it, a timer's
// expiry included: the time-triggered segment holds the join while the other
// segment waits for a signal, and the signal holds it while the other waits
// for the clock. Neither segment's effect runs.
package Test {
	private import ScalarValues::*;
	private import SI::*;

	attribute def Go;

	state Machine {
		attribute log : String = "";

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
		transition first sync do assign log := log + "sync" then done;
	}
}
