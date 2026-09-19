// A node's arguments bind its pins before the defaults it declares are evaluated:
// the default an argument replaces (`in a = 1 / 0`) is never evaluated, and a
// default the node keeps (`in b = a * 10`) reads the argument's value. The
// argument itself is the caller's expression (`seven`).
package test {
	private import ScalarValues::*;

	action def Scale {
		in a : Integer;
		in b : Integer;
		out product : Integer;

		first step;
		action step { assign product := a * b; }
	}

	action outer {
		attribute seven : Integer = 7;
		out attribute result : Integer;

		first start;
		then action scaled = Scale(a = seven) {
			in a = 1 / 0;
			in b = a * 10;
		}
		then action fin { assign result := scaled.product; }
		then done;
	}
}
