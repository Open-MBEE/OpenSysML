package test {
	private import ScalarValues::*;

	// `side` is a composite subaction no succession reaches: it starts with the
	// action, unordered against the flow `first start` begins, and the action ends
	// only once both the flow and `side` have.
	action host {
		attribute c : Integer := 0;
		attribute d : Integer := 0;
		first start;
		then action a { assign c := c + 1; }
		then done;
		action side { assign d := 7; }
	}
}
