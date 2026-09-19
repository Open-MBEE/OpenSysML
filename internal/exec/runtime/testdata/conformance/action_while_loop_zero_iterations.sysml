package test {
	private import ScalarValues::*;

	// A pre-condition loop tests before it iterates, so a condition that is
	// false on entry runs the body no times at all.
	action counter {
		attribute total : Integer = 7;
		first start;

		action accumulate {
			while total < 5 {
				assign total := total + 1;
			}
		}

		done;

		succession first start then accumulate;
		succession first accumulate then done;
	}
}
