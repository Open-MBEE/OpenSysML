package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): `a` and `b` are subactions
	// no succession reaches, so both start with `count`; `c` is reached by a
	// succession from each, so it is performed once, after both have ended.
	action count {
		attribute total : Integer := 0;
		attribute seen : Integer := 0;
		attribute cRuns : Integer := 0;
		attribute doneA : Boolean := false;
		attribute doneB : Boolean := false;
		action a { assign total := total + 1; assign doneA := true; }
		action b { assign total := total + 10; assign doneB := true; }
		action c {
			assign seen := total;
			assign cRuns := cRuns + 1;
			assign total := total + 100;
		}
		succession first a then c;
		succession first b then c;
	}
}
