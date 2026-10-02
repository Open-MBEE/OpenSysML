package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): `a` and `b` are composite
	// subactions no succession orders, so each is performed during `race`, in an
	// order the library leaves open; both increments land, whichever goes first.
	action race {
		attribute c : Integer := 0;
		action a { assign c := c + 1; }
		action b { assign c := c + 10; }
	}
}
