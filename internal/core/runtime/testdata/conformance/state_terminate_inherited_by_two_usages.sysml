// Two usages of one state definition each own a copy of the terminate action and
// the junction the definition declares, the copy nested in the composite the usage
// inherits included: the first usage's junction routes within that usage, and the
// second usage's transition terminates the machine through its own copy, so the
// exits run are the second usage's and no vertex of the first is touched.
package Test {
	private import ScalarValues::*;

	attribute def Next;
	attribute def Go;
	attribute def Abort;

	state def Inner {
		attribute log : String = "";

		entry; then idle;
		state idle {
			exit { assign log := log + "idle(exit);"; }
		}
		transition first idle accept Next then relay;
		junction relay;
		transition first relay then work;
		state work {
			exit { assign log := log + "work(exit);"; }

			entry; then w1;
			state w1 {
				exit { assign log := log + "w1(exit);"; }
			}
			transition first w1 accept Abort do { assign log := log + "abort(effect);"; } then halt;
			action halt terminate;
		}
		action stop terminate;
		transition first idle accept Abort then stop;
	}

	state def Machine {
		entry; then one;
		state one : Inner;
		state two : Inner;
		transition first one accept Go then two;
	}
}
