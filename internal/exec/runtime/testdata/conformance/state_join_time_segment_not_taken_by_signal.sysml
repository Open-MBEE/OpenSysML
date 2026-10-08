// Timer segments arrive at their own expiries and the later signal occurrence
// completes the join; dispatch order among events due together remains open.
package Test {
	private import ScalarValues::*;
	private import SI::*;

	item def Go;

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
				transition first b1 accept after 1 [s] do assign log := log + "b " then sync;
			}
			state c {
				entry; then c1;
				state c1;
				state c2;
				transition first c1 accept after 1 [s] do action { assign log := log + "c "; then send new Go() to Machine; } then c2;
			}
			state d {
				entry; then d1;
				state d1;
				state d2;
				transition first d1 accept after 1 [s] do assign log := log + "d " then d2;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
		state done;
	}
}
