package test {
	private import ScalarValues::*;

	// A decision node in an action flow, with a guarded branch and the `else`
	// branch the notation writes after it: the else branch is taken because no
	// guard out of the decision holds.
	action watchdog {
		attribute battery : Integer = 80;
		attribute alerted : Integer = 0;

		first start;

		action sendBatteryLow {
			assign alerted := 1;
		}

		action idle {
			assign alerted := 2;
		}

		done;

		succession first start then check;
		succession first sendBatteryLow then done;
		succession first idle then done;

		decide check;
		if battery < 20 then sendBatteryLow;
		else idle;
	}
}
