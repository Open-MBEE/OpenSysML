package test {
	private import ScalarValues::*;

	// `action stop terminate;` declares a terminate action usage; a token reaching
	// it through `then stop` ends the action as `then terminate;` would.
	action stopper {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;

		first start;
		then action a1 { assign x := 1; }
		then stop;
		action stop terminate;
		then action a2 { assign y := 2; }
		then done;
	}
}
