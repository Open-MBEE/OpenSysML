package Test {
	private import ScalarValues::*;

	attribute def Stop;

	// A do behavior's flow routes through a fork with a body of its own. A control
	// node's body is a step the token passing through performs, so with `Stop` in
	// the pool as `top` is entered the dispatch may cut the flow before the fork,
	// after its body, after either branch, or after both.
	state Machine {
		attribute count : Integer = 0;

		entry; then top;
		state top {
			do action run {
				first start;
				then fork split { assign count := count + 1; }
				succession first split then a;
				succession first split then b;
				action a assign count := count + 10;
				action b assign count := count + 100;
			}
		}
		transition first top accept Stop do assign count := count + 1000 then idle;
		state idle;
	}
}
