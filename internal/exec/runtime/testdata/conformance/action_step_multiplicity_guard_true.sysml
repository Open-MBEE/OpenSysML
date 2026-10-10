package test {
	private import ScalarValues::*;

	// A guarded succession into a repeated step orders its performances through
	// the written [*] target end: `g` holds and `a` performs three times.
	action def U {
		attribute c : Integer = 0;
		attribute g : Boolean = true;
		first start then p;
		action p;
		succession first p if g then [*] a;
		action a[3] { assign c := c + 1; }
		succession first [*] a then [1] done;
	}
}
