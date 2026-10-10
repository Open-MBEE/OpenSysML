package test {
	private import ScalarValues::*;

	// The same guarded succession refused by a false guard: `a`'s exact count
	// still wants three performances, which the pruned link leaves unordered
	// with respect to `p`.
	action def U {
		attribute c : Integer = 0;
		attribute g : Boolean = false;
		first start then p;
		action p;
		succession first p if g then [*] a;
		action a[3] { assign c := c + 1; }
		succession first [*] a then [1] done;
	}
}
