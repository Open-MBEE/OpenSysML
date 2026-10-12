package test {
	private import ScalarValues::*;

	item def Go;

	state def Machine {
		attribute c : Integer = 0;
		action def Bump;

		entry; then ready;
		state ready;
		state active {
			entry action mixed : Bump { assign c := c + 1; }
		}
		transition first ready accept Go then active;
	}
}
