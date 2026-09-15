// Two time-triggered segments into a join whose timers are due at different
// instants are never one occurrence: the first expiry finds the other timer
// not yet due and the second finds the first already spent, so the join does
// not fire and neither segment's effect runs.
package Test {
	private import ScalarValues::*;
	private import SI::*;

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
				transition first b1 accept after 3 [s] do assign log := log + "b " then sync;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
		state done;
	}
}
