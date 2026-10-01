// Oracle: Transitions into a join: each exits its source and runs its effect before the owner is left, in which order is open.
package Test {
	private import ScalarValues::*;
	attribute def Go;
	attribute def Prime;
	attribute def Finish;

	state Machine {
		attribute log : String = "";
		attribute open : Boolean = false;

		entry; then Owner;
		state Owner parallel {
			exit { assign log := log + "owner-exit "; }
			state left {
				entry; then a;
				state a {
					exit { assign log := log + "A-exit "; }
				}
				transition first a accept Go do assign log := log + "A-effect " then sync;
			}
			state middle {
				entry; then b;
				state b {
					exit { assign log := log + "B-exit "; }
					entry; then b1;
					state b1;
					state b2;
					transition first b1 accept Go do assign log := log + "inner " then b2;
				}
				transition first b accept Go do assign log := log + "B-effect " then sync;
			}
			state right {
				entry; then c;
				state c {
					exit { assign log := log + "C-exit "; }
				}
				transition first c accept Prime do assign log := log + "C-effect " then sync;
			}
			state gate {
				entry; then closed;
				state closed;
				state opened;
				transition first closed accept Finish do {
					assign log := log + "opened ";
					assign open := true;
				} then opened;
			}
			join sync;
			transition first sync if open do assign log := log + "out" then done;
		}
		state done;
	}
}
