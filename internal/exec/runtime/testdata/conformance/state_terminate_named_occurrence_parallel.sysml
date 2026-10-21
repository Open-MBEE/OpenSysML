// A transition in one region of a parallel machine ending at a terminate action
// naming an occurrence (`action halt terminate worker { … }`) runs the node's body,
// ends that occurrence (SysML v2 §8.3.17.16) and completes its own region, as a
// transition to `done` there does: the sibling region keeps running, still
// accepts `Ready`, and the machine completes only once that region is done too.
package test {
	private import ScalarValues::*;

	attribute def Never;
	attribute def Abort;
	attribute def Ready;

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
		attribute alarm : Integer = 0;
		attribute exits : Integer = 0;
		attribute readied : Integer = 0;

		exhibit state control parallel {
			state watch {
				entry; then busy;
				state busy {
					exit action leave { assign exits := 1; }
				}
				transition first busy accept Abort then halt;
				action halt terminate worker {
					assign alarm := 1;
				}
			}
			state serve {
				entry; then waiting;
				state waiting;
				transition first waiting accept Ready then served;
				state served {
					entry action mark { assign readied := 1; }
				}
				transition first served then done;
			}
		}
	}
}
