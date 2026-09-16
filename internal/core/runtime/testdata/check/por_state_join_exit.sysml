// A join of the machine's own regions leaves every region, the one with no
// segment into it too: its state's exit writes a feature an action reads after
// sending the join its signal, so the dispatch is dependent on the read, and each
// reading is reached.
package test {
	private import ScalarValues::*;

	item def Go;

	part def Meter {
		attribute count : Integer = 0;
	}

	part meter : Meter;

	action reader {
		attribute saw : Integer = -1;
		first start;
		then action go { send Go() to gather; }
		then action look { assign saw := meter.count; }
		then done;
	}

	state gather parallel {
		state left {
			entry; then l1;
			state l1;
			transition first l1 accept Go then sync;
		}
		state right {
			entry; then r1;
			state r1;
			transition first r1 then sync;
		}
		state third {
			entry; then t1;
			state t1 { exit { assign meter.count := 1; } }
		}
		join sync;
		transition first sync then done;
	}
}
