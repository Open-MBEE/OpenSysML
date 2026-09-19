// Entering a parallel state enters its regions as one front of units drawn
// one at a time: each region logs its own entry and then its initial state's,
// and the library orders nothing between one region's units and the other's.
// The six linearizations of two chains of two are the admissible set.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				entry { assign log := log + "left(entry) "; }
				entry; then l;
				state l { entry { assign log := log + "l(entry) "; } }
			}
			state right {
				entry { assign log := log + "right(entry) "; }
				entry; then r;
				state r { entry { assign log := log + "r(entry) "; } }
			}
		}

		transition first idle accept Go then work;
	}
	attribute def Go;
}
