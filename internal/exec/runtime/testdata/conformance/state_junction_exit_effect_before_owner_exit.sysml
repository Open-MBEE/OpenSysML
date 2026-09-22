// A junction inside a composite state stands for an exit point: the transition
// into it from a nested state runs its effect after the nested exits and
// before the owner's exit action, then the outgoing segment's effect runs.
package Test {
	private import ScalarValues::*;

	attribute def Go;

	state Machine {
		attribute log : String = "";

		entry; then S1;
		state S1 {
			exit action { assign log := log + "S1(exit);"; }
			entry; then S11;
			state S11 {
				exit action { assign log := log + "S11(exit);"; }
				entry; then S111;
				state S111 {
					exit action { assign log := log + "S111(exit);"; }
				}
			}
			junction XP;
			transition first S111 accept Go do assign log := log + "T12(effect);" then XP;
		}
		transition first S1::XP do assign log := log + "T3(effect);" then S2;
		state S2 {
			entry action { assign log := log + "S2(entry);"; }
		}
	}
}
