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
}
