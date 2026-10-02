package test {
	private import ScalarValues::*;

	item def Go;

	state def Machine {
		attribute hits : Integer = 0;
		action def BumpBase {
			first start then a;
			action a { assign hits := hits + 1; }
			then done;
		}
		action def Bump :> BumpBase;

		entry; then ready;
		state ready;
		state active {
			do action mixed : Bump {
				action b { assign hits := hits + 10; }
			}
		}
		transition first ready accept Go then active;
	}
}
