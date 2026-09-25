package test {
	private import ScalarValues::*;

	// `stop`, a terminate action usage performed as a statement of node's body, names
	// itself in its own body: its later assignment is skipped, and as a terminate action
	// usage it ends node, so `later` does not run; the action goes on to tail.
	action host {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;
		out attribute z : Integer = 0;

		first start;
		then action node {
			if true {
				action stop terminate {
					assign x := 1;
					terminate stop;
					assign y := 5;
				}
				action later { assign y := 2; }
			}
		}
		then action tail { assign z := 3; }
		then done;
	}
}
