package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): the guard on a merge's outgoing
	// succession is a DecisionTransitionAction performed after the merge performance it
	// leaves, so the merge body's write is what the guard reads. `count` performs on
	// every arrival (arrivals = 3); the third arrival's own increment turns the guard
	// false, so `work` ran only for the first two (continued = 2).
	action countArrivals {
		attribute arrivals : Integer = 0;
		attribute continued : Integer = 0;

		first start;
		merge count { assign arrivals := arrivals + 1; }
		action work { assign continued := continued + 1; }

		succession first start then count;
		succession first count if arrivals < 3 then work;
		succession first work then count;
	}
}
