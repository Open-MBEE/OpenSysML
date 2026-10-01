// Oracle: Transitions into a join: each exits its source and runs its effect before the owner is left, in which order is open.
package Test {
	private import ScalarValues::*;
	attribute def X;
	attribute def Y;
	attribute def Z;

	state Machine {
		attribute log : String = "";

		entry; then S1;
		state S1 parallel {
			state left {
				entry; then a;
				state a;
				transition first a accept X do assign log := log + "X " then sync;
			}
			state right {
				entry; then b;
				state b;
				transition first b accept Y do assign log := log + "Y " then sync;
			}
			join sync;
			transition first sync do assign log := log + "sync" then done;
		}
		state done;
		transition first S1 accept Z do assign log := log + "Z " then S1;
	}
}
