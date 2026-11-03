package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once, while `q`
	// is valued by an expression over `p`, which only a statement after the join
	// writes, from a literal: the open order touched `x` alone, and `q`, reading
	// nothing order-dependent, stands on no choice point.
	action race {
		attribute x : Integer = 0;
		attribute p : Integer = 0;
		attribute q : Integer = p + 1;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action settle { assign p := 7; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then settle;
		succession first settle then done;
	}
}
