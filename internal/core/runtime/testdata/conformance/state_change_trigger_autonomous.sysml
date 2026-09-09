// A change trigger is driven by run-to-completion itself: the condition its do
// behavior makes true is re-tested by the step after it, so the machine
// progresses without an external caller polling.
package test {
	state Machine {
		attribute log : Integer = 0;
		attribute ready : Boolean = false;

		entry; then start;
		state start;
		state waiting {
			do action arm { assign ready := true; }
		}
		accept when ready then done;
		state done {
			entry { assign log := log * 10 + 1; }
		}

		succession first start then waiting;
	}
}
