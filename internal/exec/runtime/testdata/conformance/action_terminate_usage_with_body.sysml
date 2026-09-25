package test {
	private import ScalarValues::*;

	// A terminate action usage's body runs as a leaf node's before the node ends the
	// action: its assignment and the branch it takes are observed, y is not.
	action stopper {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;
		out attribute code : Integer = 0;

		first start;
		then action a1 { assign x := 1; }
		then stop;
		action stop terminate {
			assign code := x + 41;
			if code == 42 { assign x := x + 1; }
		}
		then action a2 { assign y := 2; }
		then done;
	}
}
