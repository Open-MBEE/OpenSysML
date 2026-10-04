package test {
	private import ScalarValues::*;

	action def Order {
		attribute x : Integer := 0;
		attribute y : Integer := 0;
		first start then s;
		// Oracle (docs/project/behavior-semantic-oracle.md): the two assignments are
		// subactions of `s` no succession orders, so `y` may read `x` before or after it is written.
		action s {
			assign x := 1;
			assign y := x;
		}
		then done;
	}
}
