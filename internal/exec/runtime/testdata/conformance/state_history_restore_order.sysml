// A deep history restores the regions it recorded as one front drawn one unit
// at a time, as entering them afresh does. Each draw the machine makes is
// logged: the two regions' second states are entered on one occurrence in
// either order, exited in either order when the owner is left, and restored in
// either order; the eight combinations are the admissible set.
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
					entry { assign log := log + "l2(entry) "; }
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
