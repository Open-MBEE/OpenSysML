package test {
	private import ScalarValues::*;

	attribute def Go;

	state Machine {
		attribute parentEntered : Integer = 0;
		attribute childEntered : Integer = 0;

		entry; then idle;
		state idle;
		state target {
			entry action boot { assign parentEntered := 1; }
			transition boot then pick;
			junction pick;
			transition first pick then stop;
			action stop terminate;
			state nested {
				entry { assign childEntered := 1; }
			}
		}
		transition first idle accept Go then target;
	}
}
