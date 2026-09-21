// A join inside a composite state stands for an exit point reached from each
// orthogonal region: each incoming segment exits its region and runs its
// effect, then the owner's exit action runs, then the outgoing segment fires.
package Test {
	private import ScalarValues::*;

	attribute def Go;

	state Machine {
		attribute log : String = "";

		entry; then S1;
		state S1 parallel {
			exit action { assign log := log + "S1(exit);"; }
			state R1 {
				entry; then A1;
				state A1 { exit action { assign log := log + "A1(exit);"; } }
				transition first A1 accept Go do assign log := log + "T12(effect);" then XP;
			}
			state R2 {
				entry; then B1;
				state B1 { exit action { assign log := log + "B1(exit);"; } }
				transition first B1 accept Go do assign log := log + "T22(effect);" then XP;
			}
			join XP;
		}
		transition first S1::XP do assign log := log + "T3(effect);" then S2;
		state S2 {
			entry action { assign log := log + "S2(entry);"; }
		}
	}
}
