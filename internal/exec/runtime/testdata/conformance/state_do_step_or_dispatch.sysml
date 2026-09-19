// A state's do behavior and a dispatch due at one instant: Stop is in the pool as
// top is entered, and whether the do behavior acts before the dispatch leaves the
// state is open. Under check and explore the order is a draw at each such
// instant; the fixed policies run the do round to its end and dispatch after it.
package Test {
	private import ScalarValues::*;

	attribute def Stop;

	state Machine {
		attribute log : String = "";

		entry; then top;
		state top {
			do { assign log := log + "did "; }
		}
		transition first top accept Stop do assign log := log + "stop " then idle;
		state idle;
	}
}
