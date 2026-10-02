package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): a case body stating a
	// flow returns through the control flow ending it after the flow has run: the
	// trailing `if` is the body's result, not a step of the flow, so it reads the
	// value the step left and returns 7.
	calc def Outer {
		analysis tally {
			attribute got : Integer := 0;
			first start;
			then action a { assign got := 5; }
			if got == 5 {
				return : Integer = 7;
			}
		}
		return : Integer = tally;
	}
	analysis def Top {
		return r : Integer = Outer();
	}
	analysis top : Top;
}
