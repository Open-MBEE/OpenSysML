package test {
	private import ScalarValues::*;

	// A loop containing a conditional, and the conditional containing a loop:
	// every third count runs an inner loop that adds three to the total.
	action counter {
		attribute count : Integer = 0;
		attribute total : Integer = 0;
		attribute bursts : Integer = 0;
		first start;

		action accumulate {
			while count < 6 {
				assign count := count + 1;
				if count == 3 {
					assign bursts := bursts + 1;
					attribute inner : Integer = 0;
					while inner < 3 {
						assign inner := inner + 1;
						assign total := total + 1;
					}
				} else {
					assign total := total + 1;
				}
			}
		}

		done;

		succession first start then accumulate;
		succession first accumulate then done;
	}
}
