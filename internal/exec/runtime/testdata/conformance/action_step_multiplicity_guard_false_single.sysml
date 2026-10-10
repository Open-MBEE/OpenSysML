package test {
	private import ScalarValues::*;

	// The succession's written [1] target end counts `a`'s single performance,
	// which needs no ordering among repetitions: a false guard just prunes
	// the edge, so `a` never performs.
	action def U {
		attribute c : Integer = 0;
		first start then p;
		action p { assign c := c + 1; }
		succession first p if false then [1] a;
		action a[1] { assign c := c + 10; }
		succession first p if true then done;
	}
}
