package test {
	private import ScalarValues::*;

	action def Add {
		in x : Integer;
		out result : Integer;
		assign result := x + 1;
	}

	// Two unordered writers race on the outer `x`. `lit` performs `Add` with the literal
	// argument `x = 5`: the argument, not the outer `x` of the same name, binds the pin,
	// so `lit.x` prints bare. `dep` performs it with `x = y`, where `y` was computed from
	// the outer `x`, so `derived` stands on the writers' order as `y` does. The body of
	// `Add` reads its `x` by name, which is matched conservatively against the outer `x`,
	// so `lit.result` and `fixed` still print marked.
	action outer {
		attribute x : Integer = 0;
		attribute y : Integer = 0;
		attribute fixed : Integer = 0;
		attribute derived : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action mk { assign y := x + 10; }
		action lit = Add(x = 5);
		action dep = Add(x = y);
		action fin { assign fixed := lit; assign derived := dep; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then mk;
		succession first mk then lit;
		succession first lit then dep;
		succession first dep then fin;
		succession first fin then done;
	}
}
