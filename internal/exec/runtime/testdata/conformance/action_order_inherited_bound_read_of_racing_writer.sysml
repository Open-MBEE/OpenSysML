package test {
	private import ScalarValues::*;

	// `Base` values `y` by an expression over `x`; `race` redefines `y` without a
	// value of its own, so it is bound as `Base::y` is (KerML 1.1 §7.3.4.2), and
	// two branches between one fork and one join each write `x` once: `y`, read
	// after the join, and `z`, written from it, stand on the choice points `x`
	// does, as they would had `race` declared the expression itself.
	action def Base {
		attribute x : Integer = 0;
		attribute y : Integer = x + 1;
	}

	action race : Base {
		attribute :>> y;
		attribute z : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action derive { assign z := y; }
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
