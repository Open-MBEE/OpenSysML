package test {
	private import ScalarValues::*;

	// An unordered accept is a subperformance of the action as `b` is: the action
	// cannot end before it does, and with nothing to send its message it never does.
	action waiting {
		attribute n : Integer := 0;
		action wait accept x : Integer;
		action b { assign n := 1; }
	}
}
