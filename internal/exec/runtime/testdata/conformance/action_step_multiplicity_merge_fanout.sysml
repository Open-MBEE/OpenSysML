package test {
	private import ScalarValues::*;

	// The merge declares no multiplicity, and its ends force no other count,
	// so it takes the executor's one-performance reading — an unwritten step
	// performs once per arrival — and `a` still performs three times out of
	// `m`'s single performance through the written [*] target end, unordered
	// with respect to one another.
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
