package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `level` once; a nested
	// action then writes its own `level`, a different feature of the same name,
	// from a literal. That write leaves the outer `level` standing on the open
	// order of its writers; a same-named value elsewhere is not the one rewritten.
	action route {
		attribute level : Integer = 0;

		first start;
		fork split;
		action a { assign level := 1; }
		action b { assign level := 2; }
		join sync;
		action other {
			attribute level : Integer = 0;
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
