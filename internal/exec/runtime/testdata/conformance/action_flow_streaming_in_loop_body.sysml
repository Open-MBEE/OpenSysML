package test {
	private import ScalarValues::*;

	// A streaming flow in a loop body joins the body's nodes each pass: the value src writes
	// on pass i is what dst reads on that pass, so total = 1 + 2 + 3.
	action outer {
		out attribute total : Integer = 0;

		first start;
		then action iterate {
			for i in 1..3 {
				action src { out v : Integer; assign v := i; }
				flow src.v to dst.v;
				action dst { in v : Integer; assign total := total + v; }
			}
		}
		then done;
	}
}
