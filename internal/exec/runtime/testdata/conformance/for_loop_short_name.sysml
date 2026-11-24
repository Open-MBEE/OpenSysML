package test {
	private import ScalarValues::*;

	// A `for` variable is read in the body under its short name as under its name.
	action outer {
		out attribute total : Integer = 0;

		first start;
		then action iterate {
			for <s> i in 1..3 {
				assign total := total + s + i;
			}
		}
		then done;
	}
}
