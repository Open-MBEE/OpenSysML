// An action declared in a branch body of a calculation states a flow of its own
// from `start` to `done`: the calculation performs it, and its step is run.
package test {
	private import ScalarValues::*;

	calc c {
		attribute x : Integer = 3;
		attribute seen : Integer = 0;
		if x > 0 {
			action p {
				first start;
				then action bump { assign seen := 1; }
				then done;
			}
		}
		return : Integer = seen;
	}
}
