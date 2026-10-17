package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once; after the
	// join `x` is written again from a literal and `y` computed from the new `x`.
	// The join orders the rewrite after both racing writes, so what stands in `x`
	// is the rewrite whatever their order, and `y` is computed from it: the open
	// order touched `x`, and nothing the run ends with stands on it.
	action settle {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action reset { assign x := 10; }
		action derive { assign y := x + 1; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then reset;
		succession first reset then derive;
		succession first derive then done;
	}
}
