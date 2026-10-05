package test {
	private import ScalarValues::*;

	action def Chain {
		attribute x : Integer := 1;
		first start then s;
		// Oracle (docs/project/behavior-semantic-oracle.md): three assignments of `x` no
		// succession orders: six orders, four values.
		action s {
			assign x := x + 1;
			assign x := x * 2;
			assign x := x + 3;
		}
		then done;
	}
}
