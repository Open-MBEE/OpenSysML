package test {
	private import ScalarValues::*;

	// One fork branch terminates the action while the other is parked at an
	// accept no send ever satisfies: the parked token is dropped with the action,
	// and the node after the accept never runs.
	action racer {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;

		first start;
		fork split;
		action waiter accept sig : Integer;
		action tally { assign y := 1; }
		action quit { assign x := 1; }
		then terminate;
		succession first start then split;
		succession first split then waiter;
		succession first waiter then tally;
		succession first split then quit;
	}
}
