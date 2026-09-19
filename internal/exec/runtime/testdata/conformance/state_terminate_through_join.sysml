// A join whose outgoing segment leads into a terminate action declared beside the
// orthogonal state ends the machine once every source has arrived: the sources and
// the composite they leave are exited, as for any transition out of it, the segment's
// effect runs, and the terminate is reached; the run is terminated, at no final state.
package Test {
	private import ScalarValues::*;

	attribute def Check;

	state def Machine {
		attribute log : String = "";

		entry; then work;
		state work parallel {
			exit { assign log := log + "work(exit);"; }
			state left {
				entry; then l1;
				state l1 {
					exit { assign log := log + "l1(exit);"; }
				}
				transition first l1 accept Check then meet;
			}
			state right {
				entry; then r1;
				state r1 {
					exit { assign log := log + "r1(exit);"; }
				}
				transition first r1 accept Check then meet;
			}
		}
		join meet;
		action stop terminate;

		transition first meet do { assign log := log + "meet;"; } then stop;
	}
}
