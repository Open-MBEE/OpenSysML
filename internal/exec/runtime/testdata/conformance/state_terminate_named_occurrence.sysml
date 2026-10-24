// A transition to a terminate action naming an occurrence (`action halt
// terminate worker;`) ends that occurrence, not the containing state performance
// (SysML v2 §8.3.17.16): the worker's machine is terminated where it waits, its
// `dead` state never entered, while the controller's machine — its source state
// the final one, as for any transition target that is no state (§7.18.3) — exits
// that state and completes.
package test {
	private import ScalarValues::*;

	attribute def Never;
	attribute def Abort;

	state def Beating {
		attribute beats : Integer = 0;
		entry; then alive;
		state alive {
			entry action one { assign beats := 1; }
		}
		transition first alive accept Never then dead;
		state dead {
			entry action two { assign beats := 2; }
		}
	}

	part def Worker {
		exhibit state rhythm : Beating;
	}

	part def Controller {
		part worker : Worker;
		attribute exits : Integer = 0;

		exhibit state control {
			entry; then busy;
			state busy {
				exit action leave { assign exits := 1; }
			}
			transition first busy accept Abort then halt;
			action halt terminate worker;
		}
	}
}
