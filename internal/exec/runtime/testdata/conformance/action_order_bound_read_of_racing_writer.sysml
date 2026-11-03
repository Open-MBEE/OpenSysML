package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once, and `y` is
	// valued by an expression over `x`, computed as it is read: the library orders
	// neither write before the other, so `x` stands open, and `y`, read after the
	// join, stands on the same choice points as a value written from `x` would.
	action race {
		attribute x : Integer = 0;
		attribute y : Integer = x + 1;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
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
