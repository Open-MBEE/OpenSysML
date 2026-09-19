package test {
	private import ScalarValues::*;

	// SysML v2 7.17.3: a decision selects one outgoing succession, and a merge
	// follows one of its incoming successions.
	action route {
		attribute battery : Integer = 10;
		attribute selected : Integer = 0;

		first start;
		action low {
			assign selected := 1;
		}
		action normal {
			assign selected := 2;
		}
		merge converge;
		done;

		succession first start then choose;
		succession first low then converge;
		succession first normal then converge;
		succession first converge then done;

		decide choose;
		if battery < 20 then low;
		else normal;
	}
}
