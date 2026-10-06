package test {
	private import ScalarValues::*;

	// The rule of `action_unordered_join_once` in a do behavior: the body's `a`
	// and `b` start with it, and its `c` is performed once, after both.
	state Machine {
		attribute total : Integer = 0;
		attribute seen : Integer = 0;
		attribute cRuns : Integer = 0;
		attribute doneA : Boolean = false;
		attribute doneB : Boolean = false;
		entry; then init;
		state init;
		state active {
			do action ops {
				action a { assign total := total + 1; assign doneA := true; }
				action b { assign total := total + 10; assign doneB := true; }
				action c {
					assign seen := total;
					then assign cRuns := cRuns + 1;
					then assign total := total + 100;
				}
				succession first a then c;
				succession first b then c;
			}
		}
		succession first init then active;
		succession first active then done;
	}
}
