package test {
	private import ScalarValues::*;

	// A `ref` action usage is referential, not a composite subaction, so nothing
	// performs it; the composite `a` is performed.
	action host {
		attribute c : Integer := 0;
		ref action r { assign c := c + 100; }
		action a { assign c := c + 1; }
	}
}
