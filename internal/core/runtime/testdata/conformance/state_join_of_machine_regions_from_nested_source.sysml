// A join of the machine's own orthogonal regions, one segment leaving a state
// nested in an orthogonal state of a region: the segment exits its source, the
// orthogonal state's other region and the orthogonal state itself, once each, and
// the machine's regions are left with no state exited a second time.
package Test {
	private import ScalarValues::*;

	attribute def Go;

	state def Machine parallel {
		attribute log : String = "";

		state left {
			entry; then inner;
			state inner parallel {
				exit action { assign log := log + "inner(exit) "; }
				state l1 {
					entry; then a;
					state a {
						exit action { assign log := log + "a(exit) "; }
					}
				}
				state l2 {
					entry; then c;
					state c {
						exit action { assign log := log + "c(exit) "; }
					}
				}
			}
			transition first a accept Go do assign log := log + "a " then sync;
		}
		state right {
			entry; then b;
			state b {
				exit action { assign log := log + "b(exit) "; }
			}
			transition first b accept Go do assign log := log + "b " then sync;
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
	}
}
