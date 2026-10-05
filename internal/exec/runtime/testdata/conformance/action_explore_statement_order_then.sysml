package test {
	private import ScalarValues::*;

	action def Then {
		attribute x : Integer := 0;
		attribute y : Integer := 0;
		first start then s;
		// Oracle (docs/project/behavior-semantic-oracle.md): `then` is a succession between
		// the two assignments, so `y` reads `x` only after it is written.
		action s {
			assign x := 1;
			then assign y := x;
		}
		then done;
	}
}
