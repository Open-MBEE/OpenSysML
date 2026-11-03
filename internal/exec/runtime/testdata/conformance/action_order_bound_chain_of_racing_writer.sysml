package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once; `y` is
	// valued by an expression over `x` and `v` by one over `y`, and a statement
	// after the join writes `z` from `v`: what an expression reads is followed
	// through features valued by expressions in turn, so `v` and `z` stand on the
	// choice points `x` does.
	action race {
		attribute x : Integer = 0;
		attribute y : Integer = x + 1;
		attribute v : Integer = y * 2;
		attribute z : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action derive { assign z := v; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then derive;
		succession first derive then done;
	}
}
