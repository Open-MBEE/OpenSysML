package test {
	private import ScalarValues::*;

	// A `for` variable is one binding under its name and its short name: the body reads it
	// by either, and an assignment through one is read through the other.
	action outer {
		out attribute total : Integer = 0;

		first start;
		then action iterate {
			for <s> i in 1..3 {
				assign total := total + s + i;
			}
			then for <s> i in 1..2 {
				assign s := s * 10;
				then assign total := total + i;
				then assign i := i + 1;
				then assign total := total + s;
			}
		}
		then done;
	}
}
