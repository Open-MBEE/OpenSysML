// The transitions into a join leave states nested below the direct members of
// Work's regions. Firing the join exits each source and the composite states
// between it and Work, innermost first, before the segment's effect, then Work
// itself: every exit action runs.
package Test {
	private import ScalarValues::*;

	attribute def Go;

	state Machine {
		attribute log : String = "";

		entry; then Work;
		state Work parallel {
			exit action { assign log := log + "work(exit) "; }
			state a {
				entry; then ia;
				state ia {
					exit action { assign log := log + "ia(exit) "; }
					entry; then a1;
					state a1;
				}
				transition first a1 accept Go do assign log := log + "a1 " then sync;
			}
			state b {
				entry; then ib;
				state ib {
					exit action { assign log := log + "ib(exit) "; }
					entry; then b1;
					state b1;
				}
				transition first b1 accept Go do assign log := log + "b1 " then sync;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
		state done;
	}
}
