package test {
	private import ScalarValues::*;

	item def Go;

	state def Machine {
		attribute hits : Integer = 0;
		action def BumpBase {
			inout n : Integer;
			first start then a;
			action a { assign n := n + 1; }
			then done;
		}
		action def Bump :> BumpBase;

		entry; then ready;
		state ready;
		state active {
			entry action mixed : Bump {
				inout n = hits;
				action b { assign n := n + 10; }
			}
		}
		transition first ready accept Go then active;
	}
}
