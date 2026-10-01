// The timer-triggered segment arrives without resolving the join's outgoing
// route. When the signal-triggered segment later completes the join, its dead
// route disables that transition rather than failing the run.
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
