package test {
	private import ScalarValues::*;

	// A nested node forks in turn; its `terminate` drops the node's own parked
	// branch and ends the node, while the parent's other branch runs on and the
	// join fires as after a normal completion of the node.
	action host {
		out attribute inner : Integer = 0;
		out attribute outer : Integer = 0;
		out attribute after : Integer = 0;

		fork split;
		action node {
			fork nsplit;
			action waiter accept sig : Integer;
			action quit { assign inner := 1; }
			then terminate;
			first nsplit;
			succession first nsplit then waiter;
			succession first nsplit then quit;
		}
		action other { assign outer := 2; }
		join sync;
		action tail { assign after := 3; }
		succession first start then split;
		succession first split then node;
		succession first split then other;
		succession first node then sync;
		succession first other then sync;
		succession first sync then tail;
		succession first tail then done;
	}
}
