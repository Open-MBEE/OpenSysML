package test {
	private import ScalarValues::*;

	// A transition ends in ActionBody (SysML.xtext TransitionUsage) and names its
	// ends by feature chain, so `beta.work` is the substate it enters.
	state TestTransitionBodyDottedTarget {
		attribute marker : Integer = 0;
		attribute stepped : Integer = 0;

		entry; then start;
		state start;
		state beta {
			state work {
				entry { assign marker := 2; }
			}
		}

		transition first start then beta.work {
			assign stepped := 1;
		}
		transition first beta.work then done when Leave;
	}
}
