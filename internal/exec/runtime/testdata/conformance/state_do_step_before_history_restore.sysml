// A deep history restores two regions as one front: the do behavior of the
// state one region is restored into starts as that state is re-entered, and its
// step is drawn against the other region's restored entry as it was against the
// other region's firing on the first occurrence, where the Pause already in the
// pool may also cut it off before it moves.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then work;
		state work parallel {
			deep history resume;
			state left {
				entry; then l1;
				state l1;
				state l2 {
					do { assign log := log + "did "; }
					exit { assign log := log + "l2(exit) "; }
				}
				transition first l1 accept Go then l2;
			}
			state right {
				entry; then r1;
				state r1;
				state r2 {
					entry { assign log := log + "r2(entry) "; }
					exit { assign log := log + "r2(exit) "; }
				}
				transition first r1 accept Go then r2;
			}
		}
		state paused;

		transition first work accept Pause then paused;
		transition first paused accept Resume then resume;
	}
	attribute def Go;
	attribute def Pause;
	attribute def Resume;
}
