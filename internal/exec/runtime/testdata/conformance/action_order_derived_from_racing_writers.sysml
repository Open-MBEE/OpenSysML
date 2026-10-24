package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once, and a
	// statement after the join computes `y` from `x`: the library orders neither
	// write before the other, so `x` stands open, and `y` stands on the same
	// choice points since it was computed from `x`. `z`, written from a literal,
	// depends on no order.
	action race {
		attribute x : Integer = 0;
		attribute y : Integer = 0;
		attribute z : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action derive { assign y := x + 1; assign z := 5; }
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
