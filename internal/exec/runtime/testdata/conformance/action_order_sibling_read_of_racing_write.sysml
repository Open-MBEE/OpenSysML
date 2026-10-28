package test {
	private import ScalarValues::*;

	// Between one fork and one join, one branch writes `x` while its sibling
	// reads `x` into `z`: the library orders neither before the other, so the
	// read may see either value, and `z` stands open with `x`.
	action race {
		attribute x : Integer = 0;
		attribute z : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign z := x; }
		join sync;
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then done;
	}
}
