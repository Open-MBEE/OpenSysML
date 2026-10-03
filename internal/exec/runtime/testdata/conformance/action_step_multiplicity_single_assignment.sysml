package test {
	private import ScalarValues::*;

	action def Single {
		attribute c : Integer := 0;
		first start then a;
		// Oracle (docs/project/behavior-semantic-oracle.md): a body of one assignment
		// reads and writes c in one move, so no update is lost.
		action a[3] { assign c := c + 1; }
		then done;
	}
}
