package test {
	private import ScalarValues::*;

	// A node declared in an `if` branch is a node of the branch's own flow, and here
	// owns a flow that forks. Its `terminate node;` names the flow node the branch
	// runs in: the branch node's performance ends with what it assigned so far, the
	// waiter parked in the forked branch is dropped with it, the rest of the branch
	// and of the body are skipped, and the parent takes node's succession.
	action host {
		out attribute before : Integer = 0;
		out attribute inner : Integer = 0;
		out attribute skipped : Integer = 0;
		out attribute after : Integer = 0;

		action node {
			assign before := 1;
			if before == 1 {
				action forked {
					fork split;
					action waiter accept sig : Integer;
					action quit { assign inner := 2; terminate node; }
					first start;
					succession first start then split;
					succession first split then waiter;
					succession first split then quit;
					succession first waiter then done;
					succession first quit then done;
				}
				action later { assign skipped := 3; }
			}
			assign skipped := 4;
		}
		action tail { assign after := 5; }
		succession first start then node;
		succession first node then tail;
		succession first tail then done;
	}
}
