// A timer-triggered segment fires at its expiry and waits at the join; a later
// signal-triggered segment completes the join on its own occurrence.
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
