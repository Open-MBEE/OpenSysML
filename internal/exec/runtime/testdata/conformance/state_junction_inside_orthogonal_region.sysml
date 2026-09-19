// A transition into a junction declared in one region of a parallel state enters
// the parallel state first: the segment out of the junction runs its effect
// after the parallel state's entry, as that region is entered, while the other
// region starts as usual.
package Test {
	state Machine {
		attribute log : String = "";
		attribute x : Integer = 0;

		entry; then idle;
		state idle;
		state both parallel {
			entry { assign log := log + "both(entry);"; }
			exit { assign log := log + "both(exit);"; }
			state left {
				entry; then l0;
				state l0 { entry { assign log := log + "l0(entry);"; } }
				junction split;
				state l1 { entry { assign log := log + "l1(entry);"; } }

				transition first split if x == 1 then l0;
				transition first split do { assign log := log + "split(effect);"; } then l1;
			}
			state right {
				entry; then r0;
				state r0 { entry { assign log := log + "r0(entry);"; } }
			}
		}

		transition first idle accept Go do { assign log := log + "go(effect);"; } then split;
		transition first both accept Stop then done;
	}
	attribute def Go;
	attribute def Stop;
}
