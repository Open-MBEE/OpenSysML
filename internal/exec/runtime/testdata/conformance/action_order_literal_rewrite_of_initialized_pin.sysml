package test {
	private import ScalarValues::*;

	// Two unordered writers race on `x`. The nested action `other` initializes its own
	// `level` from `x`, so the initializer stands on the writers' order, then rewrites
	// `level` from a literal: the rewrite is of the same feature of the same node, so
	// it clears the initializer's mark and `other.level` prints bare while `x` stays
	// order-dependent.
	action route {
		attribute x : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action other {
			attribute level : Integer = x;
			assign level := 5;
		}
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then other;
		succession first other then done;
	}
}
