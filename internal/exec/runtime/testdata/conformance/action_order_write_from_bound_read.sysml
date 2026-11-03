package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once; `y` is
	// valued by an expression over `x`, and a statement after the join writes `z`
	// from `y`: the write stands on the choice points `y`'s expression read, as a
	// write from `x` itself would. `w`, given `x + 1` as its initial value, is
	// evaluated once as the action starts, before either write, and stands on
	// nothing.
	action race {
		attribute x : Integer = 0;
		attribute y : Integer = x + 1;
		attribute w : Integer := x + 1;
		attribute z : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action derive { assign z := y + 10; }
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
