package test {
	private import ScalarValues::*;

	// Named arguments bind the callee's input parameters by the callee's own
	// parameter names, in whatever order they are written. Read as a value, the
	// node is the callee's `result`.
	action def Subtract {
		in minuend : Integer;
		in subtrahend : Integer;
		out result : Integer;

		first step;
		action step { assign result := minuend - subtrahend; }
	}

	action outer {
		out attribute difference : Integer;

		first start;
		then action diff = Subtract(subtrahend = 4, minuend = 10);
		then action fin { assign difference := diff; }
		then done;
	}
}
