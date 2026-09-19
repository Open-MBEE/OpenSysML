// The transitions into a join leave composite states whose substates are active:
// each segment is enabled by the occurrence reaching its source from within, and
// firing the join exits the active substate, then the source, then its effect.
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
					state a1 {
						exit action { assign log := log + "a1(exit) "; }
					}
				}
				transition first ia accept Go do assign log := log + "a " then sync;
			}
			state b {
				entry; then b1;
				state b1 {
					exit action { assign log := log + "b1(exit) "; }
				}
				transition first b1 accept Go do assign log := log + "b " then sync;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
		state done;
	}
}
