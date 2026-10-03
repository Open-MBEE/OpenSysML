package test {
	private import ScalarValues::*;

	// A merge declares no multiplicity of its own, and a succession out of it
	// does not fix its count: `a`'s three performances cannot order with
	// respect to `m`, so the flow is refused rather than fanned out of one
	// merge performance.
	action def U {
		attribute c : Integer = 0;
		first start then p;
		action p { assign c := c + 1; }
		merge m;
		first p then m;
		action a[3] { assign c := c + 10; }
		succession first m then [*] a;
		then done;
	}
}
