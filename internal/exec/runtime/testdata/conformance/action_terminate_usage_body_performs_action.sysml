package test {
	private import ScalarValues::*;

	action def Cleanup {
		out cleaned : Integer;
		first start;
		then action wipe { assign cleaned := 5; }
		then done;
	}

	// A terminate action usage's body performs an action before the node ends the
	// action: the performance runs to completion, its `out cleaned` comes back to
	// stopper's own, and the statement after it still runs. y is not assigned.
	action stopper {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;
		out attribute cleaned : Integer = 0;

		first start;
		then action a1 { assign x := 1; }
		then stop;
		action stop terminate {
			perform action cleanup : Cleanup;
			assign x := x + cleaned;
		}
		then action a2 { assign y := 2; }
		then done;
	}
}
