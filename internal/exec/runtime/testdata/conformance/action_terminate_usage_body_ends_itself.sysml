package test {
	private import ScalarValues::*;

	// A `terminate;` in a terminate action usage's body ends the usage's own performance
	// where it stands — the assignment after it is skipped — and the usage still ends the
	// action it is a step of: a2 does not run.
	action stopper {
		out attribute x : Integer = 0;
		out attribute y : Integer = 0;

		first start;
		then action a1 { assign x := 1; }
		then stop;
		action stop terminate {
			assign x := x + 1;
			terminate;
			assign y := 5;
		}
		then action a2 { assign y := 2; }
		then done;
	}
}
