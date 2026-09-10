package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): one fork branch is two
	// nodes, `left1` writing `x` and `left2` reading it into `y`; the other branch,
	// `right`, writes `x` once. The library orders `left2` after `left1` and nothing
	// against `right`, so `right` may run before both, between them, or after both:
	// three linearizations, and each leaves a different pair of values.
	action overtake {
		attribute x : Integer = 0;
		attribute y : Integer = 0;

		first start;
		fork split;
		action left1 { assign x := 1; }
		action left2 { assign y := x; }
		action right { assign x := 2; }
		join sync;
		done;

		succession first start then split;
		succession first split then left1;
		succession first left1 then left2;
		succession first split then right;
		succession first left2 then sync;
		succession first right then sync;
		succession first sync then done;
	}
}
