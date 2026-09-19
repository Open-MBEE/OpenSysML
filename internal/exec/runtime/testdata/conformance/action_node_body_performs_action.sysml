package test {
	private import ScalarValues::*;

	action def Cleanup {
		out cleaned : Integer;
		first start;
		then action wipe { assign cleaned := 5; }
		then done;
	}

	// A leaf action node's body performs an action among its statements: the
	// performance runs where it is written, its `out cleaned` comes back to the
	// host's own, and the statement after it reads what came back.
	action host {
		out attribute x : Integer = 0;
		out attribute cleaned : Integer = 0;

		first start;
		then action a1 {
			assign x := 1;
			perform action cleanup : Cleanup;
			assign x := x + cleaned;
		}
		then action a2 { assign x := x * 10; }
		then done;
	}
}
