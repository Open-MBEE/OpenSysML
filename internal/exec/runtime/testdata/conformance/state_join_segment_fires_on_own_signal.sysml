package Test {
	private import ScalarValues::*;
	attribute def X;
	attribute def Y;

	state Machine {
		attribute log : String = "";

		entry; then S1;
		state S1 parallel {
			exit { assign log := log + "owner-exit "; }
			state left {
				entry; then a;
				state a {
					exit { assign log := log + "X-exit "; }
				}
				transition first a accept X do assign log := log + "X-effect " then sync;
			}
			state right {
				entry; then b;
				state b {
					exit { assign log := log + "Y-exit "; }
				}
				transition first b accept Y do assign log := log + "Y-effect " then sync;
			}
			join sync;
			transition first sync do assign log := log + "out" then done;
		}
		state done;
	}
}
