// A transition to a terminate action leaves its source as any transition does
// (its exit behavior runs), performs its effect, then ends the state machine
// performance at once: nothing else is exited and the machine's own do behavior
// is aborted where it waits.
package test {
	private import ScalarValues::*;

	attribute def Never;
	attribute def Abort;

	state def Machine {
		attribute count : Integer = 0;
		attribute exits : Integer = 0;
		attribute effects : Integer = 0;

		do action tick {
			first start;
			then action a assign count := 1;
			then action wait accept Never;
			then action b assign count := 2;
			then done;
		}

		entry; then busy;

		state busy {
			exit action leave {
				assign exits := 1;
			}
		}

		transition first busy accept Abort do assign effects := 7 then stop;
		action stop terminate;
	}
}
