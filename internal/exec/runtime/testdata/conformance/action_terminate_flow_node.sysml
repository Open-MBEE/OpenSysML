package test {
	private import ScalarValues::*;

	// `then terminate;` written in the flow ends the action itself: the outputs
	// assigned so far stand, and the nodes after it never run.
	action guarded {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;

		first start;
		then action a1 { assign x := 1; }
		then terminate;
		then action a2 { assign y := 2; }
		then done;
	}
}
