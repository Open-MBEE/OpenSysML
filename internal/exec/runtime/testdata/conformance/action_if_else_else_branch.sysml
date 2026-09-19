package test {
	private import ScalarValues::*;

	// The same conditional with a condition that fails takes the else branch.
	action decide {
		attribute total : Integer = -1;
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
