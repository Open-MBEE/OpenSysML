package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): two branches between one
	// fork and one join each write a feature of their own, and neither reads the
	// other's. The library orders neither branch before the other, yet every
	// order leaves x = 1 and y = 2: the branches commute, so the run's order
	// among them decides nothing and the result is not order-dependent.
	action branches {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign y := 2; }
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
