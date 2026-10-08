package test {
	private import ScalarValues::*;

	action def ReadFirst {
		attribute x : Integer := 0;
		attribute y : Integer := 0;
		first start then s;
		// Oracle (docs/project/behavior-semantic-oracle.md): the read is declared before the
		// write, but no succession orders them, so `y` may see `x` written.
		action s {
			assign y := x;
			assign x := 1;
		}
		then done;
	}
}
