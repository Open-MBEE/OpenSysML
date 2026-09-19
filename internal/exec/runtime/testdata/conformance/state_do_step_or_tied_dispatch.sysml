// A do step and two time triggers due at one instant, one of them guarded on what
// the do step writes: before the step only the unguarded trigger would fire, so
// under check and explore it alone is drawn against the step; the guarded trigger,
// which the dispatch would drop, waits for the round to close as under the fixed
// policies, where the round runs first and the tied dispatch is drawn after it.
package Test {
	private import ScalarValues::*;
	private import SI::*;

	state Machine {
		attribute log : String = "";

		entry; then top;
		state top {
			do action mark {
				first start;
				then action wait accept after 2 [s];
				then action write assign log := log + "did ";
				then done;
			}
		}
		transition first top accept after 2 [s] if log != "" do assign log := log + "one " then idle;
		transition first top accept after 2 [s] do assign log := log + "two " then idle;
		state idle;
	}
}
