// A route through two junctions, each declared in a composite state nested one
// level deeper, runs each segment's effect after the entry of the composite
// declaring the junction it leaves: outer's entry, the segment out of its
// junction, inner's entry, the segment out of inner's junction, then the target.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state outer {
			entry { assign log := log + "outer(entry);"; }
			exit { assign log := log + "outer(exit);"; }
			junction j1;
			state inner {
				entry { assign log := log + "inner(entry);"; }
				exit { assign log := log + "inner(exit);"; }
				junction j2;
				state leaf { entry { assign log := log + "leaf(entry);"; } }

				transition first j2 do { assign log := log + "j2(effect);"; } then leaf;
			}

			transition first j1 do { assign log := log + "j1(effect);"; } then j2;
		}

		transition first idle accept Go do { assign log := log + "go(effect);"; } then j1;
		transition first outer accept Stop then done;
	}
	attribute def Go;
	attribute def Stop;
}
