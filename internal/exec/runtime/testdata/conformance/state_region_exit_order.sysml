// Leaving a parallel state exits its regions as one front of units drawn one
// at a time: a nested state's exit precedes its parent's, and the library
// orders nothing between one region's exits and the other's. The three
// linearizations of a chain of one and a chain of two are the admissible set,
// each followed by the owner's exit.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then work;
		state work parallel {
			exit { assign log := log + "work(exit) "; }
			state left {
				entry; then l;
				state l { exit { assign log := log + "l(exit) "; } }
			}
			state right {
				entry; then outer;
				state outer {
					exit { assign log := log + "outer(exit) "; }
					entry; then r;
					state r { exit { assign log := log + "r(exit) "; } }
				}
			}
		}
		state rest;

		transition first work accept Go then rest;
	}
	attribute def Go;
}
