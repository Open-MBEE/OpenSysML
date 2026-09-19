package test {
	private import ScalarValues::*;

	// A node's body resolves names lexically: a name the node declares is its own
	// pin, and a name the enclosing action declares is the enclosing action's
	// feature, written in place.
	action outer {
		attribute count : Integer = 0;
		out attribute total : Integer;

		first start;
		then action p {
			out v : Integer;
			assign v := 2;
			assign count := count + 1;
			assign total := v * 10;
		}
		then action q {
			out v : Integer;
			assign v := 5;
			assign count := count + 1;
		}
		then done;
	}
}
