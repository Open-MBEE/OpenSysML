package test {
	private import ScalarValues::*;

	// Positional arguments bind the callee's input parameters in the order the
	// callee declares them — not by looking the parameters' names up among the
	// caller's features, which here hold other values under the same names. Read
	// as a value, the node is the callee's `result`.
	action def Scale {
		in a : Integer;
		in b : Integer;
		out result : Integer;

		first step;
		action step { assign result := a * b; }
	}

	action outer {
		attribute a : Integer = 100;
		attribute b : Integer = 100;
		out attribute total : Integer;

		first start;
		then action scaled = Scale(3, 4);
		then action fin { assign total := scaled; }
		then done;
	}
}
