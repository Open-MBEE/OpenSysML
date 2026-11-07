package test {
	private import ScalarValues::*;

	// `Base` values its output `yBase` by an expression over `x`; `Race` declares
	// an output at the same position with no value and no `:>>`, so it redefines
	// `yBase` by position (KerML 8.4.4.6), is bound as `Base::yBase` is, and is
	// the only output there (the redefined one is not inherited). Two branches
	// between one fork and one join each
	// write `x` once: the output, read after the join, and `z`, written from it,
	// stand on the choice points `x` does.
	action def Base {
		attribute x : Integer = 0;
		out yBase : Integer = x + 1;
	}

	action def Race :> Base {
		out yRace : Integer;
		attribute z : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action derive { assign z := yRace; }
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
