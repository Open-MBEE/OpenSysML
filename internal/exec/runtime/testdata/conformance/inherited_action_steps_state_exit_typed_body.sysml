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

		entry; then active;
		state active {
			exit action mixed : Bump {
				action b { assign hits := hits + 10; }
			}
		}
		state completed;
		transition first active accept Go then completed;
	}
}
