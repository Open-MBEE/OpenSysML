package test {
	private import ScalarValues::*;

	// Oracle (docs/project/behavior-semantic-oracle.md): the merge is re-entered
	// after each `addCharge`, so monitor runs three times and the loop ends at 100.
	action charge {
		attribute level : Integer = 0;
		attribute passes : Integer = 0;

		first start;
		then merge continueCharging;
		then action monitor { assign passes := passes + 1; }
		then decide;
			if level < 100 then addCharge;
			if level >= 100 then endCharging;

		action addCharge { assign level := level + 50; }
		then continueCharging;

		action endCharging;
		then done;
	}
}
