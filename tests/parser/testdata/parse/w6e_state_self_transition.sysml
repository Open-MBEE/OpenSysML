package Test {
	state Machine {
		attribute log : Integer = 0;

		entry; then s;
		state s {
			entry { assign log := log * 10 + 1; }
			exit { assign log := log * 10 + 2; }
		}

		transition first s accept again do assign log := log * 10 + 9 then s;
		transition sameEnds first s accept stay then s;
	}
}
