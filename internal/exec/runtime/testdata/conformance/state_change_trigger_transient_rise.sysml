package test {
	attribute def Go;

	state Machine {
		attribute ready : Boolean = false;
		attribute log : Integer = 0;

		entry; then outer;
		state outer {
			entry; then waiting;
			state waiting;
			state pulsing;
			transition first waiting accept Go do action pulse {
				assign ready := true;
				assign ready := false;
			} then pulsing;
		}
		state done;

		transition first outer accept when ready do assign log := log + 1 then done;
	}
}
