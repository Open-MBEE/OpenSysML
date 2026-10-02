package test {
	private import ScalarValues::*;

	item def Go;

	state def Machine {
		attribute hits : Integer = 0;
		action def Base { assign hits := hits + 1; }
		action def P0 :> Base;
		entry; then active;
		state active {
			exit action run : P0;
		}
		state completed;
		transition first active accept Go then completed;
	}
}
