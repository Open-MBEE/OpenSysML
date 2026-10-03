package test {
	private import ScalarValues::*;

	// A `for` loop variable declared with only a short name is bound and
	// referenced by that name.
	action integrate {
		attribute steps = (1.0, 2.0, 3.5);
		attribute total : Real = 0.0;
		first start;

		action accumulate {
			for <s> in steps {
				assign total := total + s;
			}
		}

		done;

		succession first start then accumulate;
		succession first accumulate then done;
	}
}
