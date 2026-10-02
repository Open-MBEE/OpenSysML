package test {
	private import ScalarValues::*;

	// The rule of `action_unordered_join_once` inside an action node: `inner`'s
	// `a` and `b` start with `inner`, and its `c` is performed once, after both.
	action count {
		attribute total : Integer := 0;
		attribute seen : Integer := 0;
		attribute cRuns : Integer := 0;
		attribute doneA : Boolean := false;
		attribute doneB : Boolean := false;
		action inner {
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
}
