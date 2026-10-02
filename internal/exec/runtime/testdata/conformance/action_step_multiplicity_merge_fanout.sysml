package test {
	private import ScalarValues::*;

	// The written [*] target end admits every performance of `a` out of the
	// merge's single performance: `m` fires once and `a` still performs three
	// times, unordered with respect to one another.
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
