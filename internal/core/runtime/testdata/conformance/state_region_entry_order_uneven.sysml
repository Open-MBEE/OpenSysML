// The regions of a parallel state are entered as one front of units drawn one
// at a time, whatever their lengths: one region logs a single entry, the other
// its own entry and then its initial state's, and the library orders nothing
// between the two chains. The three linearizations of a chain of one and a
// chain of two are the admissible set.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state work parallel {
			state left {
				entry { assign log := log + "left(entry) "; }
				entry; then l;
				state l;
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
