// Deliberately invalid in one place: `pick` is a choice in the body of a parallel machine, which no transition may leave, so it routes nowhere — the error is kept, as the fixture pins where such an inherited body pseudostate is located, not how it routes.
package Regions {
	// A parallel machine whose regions are synthesized from its substates, one of
	// which holds a parallel composite state with regions of its own.
	state def Controller parallel {
		// A pseudostate of the machine's own body, which no state owns.
		choice pick;
		state sensing {
			entry; then idle;
			state idle;
			state busy parallel {
				state pumps {
					entry; then off;
					state off;
					state on;
				}
				state valves {
					entry; then shut;
					state shut;
					state open;
				}
			}
			transition first idle then busy;
		}
		state acting {
			entry; then waiting;
			state waiting;
		}
	}

	// A definition a state usage elsewhere is typed by, with a pseudostate nested
	// in its body and one in a composite state of it.
	state def Running {
		entry; then go;
		state go {
			entry; then ready;
			state ready;
			junction settle;
			transition first ready then settle;
			transition first settle then ready;
		}
		choice retry;
		transition first go then retry;
		transition first retry then go;
	}
}
