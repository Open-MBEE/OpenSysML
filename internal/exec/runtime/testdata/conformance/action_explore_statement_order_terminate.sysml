package test {
	private import ScalarValues::*;

	action def TerminateOrder {
		attribute x : Integer := 0;
		attribute z : Integer := 0;
		first start then s;
		// Oracle (docs/project/behavior-semantic-oracle.md): `terminate;` is a subaction no
		// succession orders after the assignment, so it may end `s` before `x` is written.
		action s {
			assign x := 1;
			terminate;
		}
		then action next { assign z := 3; }
		then done;
	}
}
