// Each performance evaluates its node's defaults afresh: a calc usage a default
// reads answers from the enclosing action's state as it stands then, not from
// what an earlier performance's default read.
package test {
	private import ScalarValues::*;

	calc def Double {
		in x : Integer;
		out d : Integer = x * 2;
	}

	action def Step {
		in v : Integer;
		out w : Integer;

		first copy;
		action copy { assign w := v; }
	}

	action outer {
		attribute n : Integer = 1;
		out attribute total : Integer = 0;
		calc dbl : Double { in x = n; }

		first start;
		then action first_ : Step { in v = dbl.d; }
		then action bump { assign n := n + 1; }
		then action second : Step { in v = dbl.d; }
		then action tally { assign total := first_.w + second.w; }
		then done;
	}
}
