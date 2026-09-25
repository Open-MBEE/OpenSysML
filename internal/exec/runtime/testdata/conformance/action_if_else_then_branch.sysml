package test {
	private import ScalarValues::*;

	// The then branch runs and the else branch does not, so only one of the two
	// assignments is observable.
	action decide {
		attribute total : Integer = 1;
		attribute taken : String = "none";
		first start;

		action choose {
			if total > 0 {
				assign taken := "then";
				assign total := total + 10;
			} else {
				assign taken := "else";
				assign total := total - 10;
			}
		}

		done;

		succession first start then choose;
		succession first choose then done;
	}
}
