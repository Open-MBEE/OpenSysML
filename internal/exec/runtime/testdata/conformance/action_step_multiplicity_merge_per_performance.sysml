package test {
	private import ScalarValues::*;

	// A lone succession into a merge is bijective over a repeated step's
	// performances: `m` performs once per performance rather than behind a
	// barrier, so each of `a`'s three performances traverses it.
	action def U {
		attribute c : Integer = 0;
		first start then a;
		action a[3] { assign c := c + 1; }
		merge m;
		succession first a then m;
		then done;
	}
}