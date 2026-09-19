// A typed node in a state's body binds its arguments before evaluating the
// defaults it declares: the default an argument replaces (`in a = 1 / 0`) is
// never evaluated, a default the node keeps (`in b = a * 10`) reads the
// argument's value, and the body reads both.
package test {
	private import ScalarValues::*;

	action def Scale {
		in a : Integer;
		in b : Integer;
		out product : Integer;

		first step;
		action step { assign product := a * b; }
	}

	state def Machine {
		attribute seven : Integer = 7;
		attribute product : Integer = 0;
		attribute seen : Integer = 0;

		entry; then init;
		state init;

		state active {
			entry action {
				if seen == 0 {
					action scaled = Scale(a = seven) {
						in a = 1 / 0;
						in b = a * 10;
						assign seen := a * 100 + b;
					}
				}
			}
		}

		succession first init then active;
		succession first active then done;
	}
}
