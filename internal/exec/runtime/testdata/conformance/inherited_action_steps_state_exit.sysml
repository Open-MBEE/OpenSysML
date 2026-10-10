package test {
	private import ScalarValues::*;

	item def Go;

	state def Machine {
		attribute hits : Integer = 0;
		action def Base {
			inout n : Integer;
			assign n := n + 1;
		}
		action def P0 :> Base;
		entry; then active;
		state active {
			exit action run : P0 { inout n = hits; }
		}
		state completed;
		transition first active accept Go then completed;
	}
}
