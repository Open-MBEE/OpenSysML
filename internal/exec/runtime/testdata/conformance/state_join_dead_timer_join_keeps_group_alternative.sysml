// Oracle: same-instant timer expiries are separate join occurrences, and a dead
// completing segment leaves its source's same-group alternative available.
package Test {
	private import ScalarValues::*;
	private import SI::*;

	state Machine {
		attribute log : String = "";
		attribute open : Boolean = false;

		entry; then Owner;
		state Owner parallel {
			state left {
				entry; then a;
				state a;
				transition first a accept after 1 [s] do assign log := log + "A-segment " then sync;
			}
			state right {
				entry; then b;
				state b;
				state fallback;
				transition first b accept after 1 [s] do assign log := log + "B-segment " then sync;
				transition first b accept after 1 [s] do assign log := log + "fallback " then fallback;
			}
		}
		join sync;
		transition first sync if open do assign log := log + "sync" then done;
		state done;
	}
}
