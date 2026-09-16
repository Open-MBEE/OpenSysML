// A deep history's default transition leads through a junction declared in a nested
// composite: the nested composite is entered before the junction's segment runs
// its effect, as the history's owner was before the default transition's. The
// second visit restores the leaf and takes neither segment.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle;
		state outer {
			entry { assign log := log + "outer(entry);"; }
			exit { assign log := log + "outer(exit);"; }
			deep history hist;
			junction j;
			state inner {
				entry { assign log := log + "inner(entry);"; }
				junction ij;
				state leaf { entry { assign log := log + "leaf(entry);"; } }

				transition first ij do { assign log := log + "ij(effect);"; } then leaf;
			}

			transition first hist do { assign log := log + "h(effect);"; } then j;
			transition first j do { assign log := log + "j(effect);"; } then inner.ij;
		}

		transition first idle accept Go do { assign log := log + "go(effect);"; } then outer.hist;
		transition first outer accept Out then idle;
		transition first outer accept Stop then done;
	}
	attribute def Go;
	attribute def Out;
	attribute def Stop;
}
