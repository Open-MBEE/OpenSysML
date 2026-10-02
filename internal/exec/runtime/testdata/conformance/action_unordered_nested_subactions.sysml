package test {
	private import ScalarValues::*;

	// `outer`'s subactions are unordered within it, and `outer` ends only after
	// both have: `later`, which `outer` precedes, reads both increments.
	action host {
		attribute c : Integer := 0;
		attribute d : Integer := 0;
		first start;
		then action outer {
			action a { assign c := c + 1; }
			action b { assign c := c + 10; }
		}
		then action later { assign d := c; }
		then done;
	}
}
