package test {
	private import ScalarValues::*;

	// A loop whose condition never fails ends the execution with the step
	// budget's diagnostic instead of running forever.
	action runaway {
		attribute total : Integer = 0;
		first start;

		action spin {
			while total >= 0 {
				assign total := total + 1;
			}
		}

		done;

		succession first start then spin;
		succession first spin then done;
	}
}
