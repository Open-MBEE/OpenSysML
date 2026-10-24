package test {
	private import ScalarValues::*;

	action increment {
		in x : Integer;
		out result : Integer;

		first begin;

		action bump {
			assign result := x + 5;
		}

		done;

		succession first begin then bump;
		succession first bump then done;
	}

	// Two branches between one fork and one join each write `x` once; a performed
	// action then takes `x` as its input and computes its output from it, which
	// the outer `result` is bound to: `result` stands on the open order of the
	// writes as `x` does, while `other`, written from a literal, is fixed.
	action outer {
		attribute x : Integer = 0;
		attribute result : Integer = 0;
		attribute other : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		perform action doIt references increment;
		action keep { assign other := 3; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then doIt;
		succession first doIt then keep;
		succession first keep then done;
	}
}
