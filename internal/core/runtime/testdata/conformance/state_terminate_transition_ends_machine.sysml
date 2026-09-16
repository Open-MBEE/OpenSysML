// A transition to a terminate action ends the state machine performance at
// once: no state is exited, no exit behavior runs, the do behavior is aborted.
package test {
	private import ScalarValues::*;

	attribute def Never;
	attribute def Abort;

	state def Machine {
		attribute count : Integer = 0;
		attribute exits : Integer = 0;
		attribute effects : Integer = 0;

		entry; then busy;

		state busy {
			do action tick {
				first start;
				then action a assign count := 1;
				then action wait accept Never;
				then action b assign count := 2;
				then done;
			}
			exit action leave {
				assign exits := 1;
			}
		}

		transition first busy accept Abort do assign effects := 7 then stop;
		action stop terminate;
	}
}
