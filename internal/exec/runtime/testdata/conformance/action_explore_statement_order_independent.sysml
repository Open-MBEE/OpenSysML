package test {
	private import ScalarValues::*;

	action def Independent {
		attribute x : Integer := 0;
		attribute y : Integer := 0;
		attribute z : Integer := 0;
		first start then s;
		// Oracle (docs/project/behavior-semantic-oracle.md): no succession orders the three
		// assignments, but none reads or writes what another does, so every order agrees.
		action s {
			assign x := 1;
			assign y := 2;
			assign z := 3;
		}
		then done;
	}
}
