package test {
	item def Start;
	item def Stop;

	state Driver {
		entry; then idle;
		state idle;
		state heating;
		transition first idle accept Start then heating;
		transition first heating accept Stop then idle;
	}
}
