package test {
	private import ScalarValues::*;

	// A `for` loop runs its body once per element of the sequence it iterates
	// over, with the element bound to the loop's own variable.
	action integrate {
		attribute steps = (1.0, 2.0, 3.5);
		attribute total : Real = 0.0;
		attribute visited : Integer = 0;
		first start;

		action accumulate {
			for s in steps {
				assign total := total + s;
				assign visited := visited + 1;
			}
		}

		done;

		succession first start then accumulate;
		succession first accumulate then done;
	}
}
