package test {
	private import ScalarValues::*;

	// Two unordered writers race on `x`. `step` writes its own output pin `v` from `x`,
	// and `finish` reads that pin as `step.v`: the read is of `step`'s pin, not of a
	// pin of `finish`, so `result` stands on the writers' order as `step.v` does.
	action outer {
		attribute x : Integer = 0;
		attribute result : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action step { out v : Integer; assign v := x; }
		action finish { assign result := step.v; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then step;
		succession first step then finish;
		succession first finish then done;
	}
}
