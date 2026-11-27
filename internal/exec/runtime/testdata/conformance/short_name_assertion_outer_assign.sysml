package test {
	private import ScalarValues::*;

	// An assertion's performance writes no feature of the action around it, by its
	// short name either: s names the action's x, which the assertion's own x is not.
	action outer {
		out attribute far : Integer = 0;
		attribute <s> x : Integer := 1;
		assert constraint c { attribute x : Integer = 100; assign s := 5; x == 100 }
		action fin { assign far := x; }
		first start then c;
		first c then fin;
		first fin then done;
	}
}
