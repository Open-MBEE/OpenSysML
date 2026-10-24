package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join both write `x`; only one writes
	// `p`. After the join, `y` is computed from `x` and `q` from `p`: `x` and `y`
	// stand on the open order of the writes, while `p` and `q`, which no other
	// branch reads or writes, are fixed whatever the order.
	action race {
		attribute x : Integer = 0;
		attribute p : Integer = 0;
		attribute y : Integer = 0;
		attribute q : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; assign p := 3; }
		action b { assign x := 2; }
		join sync;
		action derive { assign y := x + 1; assign q := p + 1; }
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
