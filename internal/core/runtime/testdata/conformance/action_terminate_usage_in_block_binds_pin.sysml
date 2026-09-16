package test {
	private import ScalarValues::*;

	// `stop`, a terminate action usage performed as a statement of node's body, completes
	// with its pins before it ends node: `code` reaches host's `result` through the branch's
	// binding, `later` does not run, and the action goes on to tail.
	action host {
		out attribute result : Integer = 0;
		out attribute y : Integer = 0;
		out attribute z : Integer = 0;

		first start;
		then action node {
			if true {
				bind stop.code = result;
				action stop terminate {
					out code : Integer;
					assign code := 42;
				}
				action later { assign y := 2; }
			}
		}
		then action tail { assign z := 3; }
		then done;
	}
}
