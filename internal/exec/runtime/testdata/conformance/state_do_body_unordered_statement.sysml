package test {
	private import ScalarValues::*;

	// A do behavior's loop body states a flow (`first a then b`); the assignment
	// no succession reaches is a subaction of the body beside it, performed once.
	state Machine {
		attribute n : Integer = 0;
		entry; then active;
		state active {
			do action work {
				for i in 1..1 {
					action a { assign n := n + 1; }
					action b { assign n := n + 10; }
					assign n := n + 100;
					first a then b;
				}
			}
		}
	}
}
