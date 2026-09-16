// Two time-triggered segments into a join whose timers are due at one instant:
// their expiries are one occurrence for the join, whichever is dispatched
// first, so the join fires, each segment's effect running before the owner is
// left, in either order, and the other timer is withdrawn with its source.
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
				transition first b1 accept after 1 [s] do assign log := log + "b " then sync;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
		state done;
	}
}
