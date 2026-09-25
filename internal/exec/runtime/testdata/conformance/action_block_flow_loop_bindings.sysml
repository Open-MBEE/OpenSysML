package test {
	private import ScalarValues::*;

	// A loop body's `bind`/`flow` address its own action nodes' pins each iteration:
	// dbl.a and dbl.b are seeded from i and dbl.sum flows to acc.n, so total = 2 + 4 + 6.
	action def Adder {
		in a : Integer;
		in b : Integer;
		out sum : Integer;

		first step;
		action step { assign sum := a + b; }
	}

	action outer {
		out attribute total : Integer = 0;

		first start;
		then action iterate {
			for i in 1..3 {
				bind dbl.a = i;
				bind dbl.b = i;
				action dbl : Adder;
				flow dbl.sum to acc.n;
				action acc { in n : Integer; assign total := total + n; }
			}
		}
		then done;
	}
}
