package test {
	private import ScalarValues::*;

	// A conditional without an else branch does nothing when its condition
	// fails, and the statements after it still run.
	action decide {
		attribute total : Integer = 0;
		first start;

		action choose {
			if total > 0 {
				assign total := total + 10;
			}
			assign total := total + 1;
		}

		done;

		succession first start then choose;
		succession first choose then done;
	}
}
