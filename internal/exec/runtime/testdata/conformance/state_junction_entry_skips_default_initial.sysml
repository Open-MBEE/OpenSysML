// A junction inside a composite state stands for an entry point: entering
// through it runs the owner's entry action, then the junction's outgoing
// effect, then the target's entry, and the owner's default initial does not
// fire. A later entry through history restores the state the junction reached.
package Test {
	private import ScalarValues::*;

	attribute def Go; attribute def Back; attribute def Again;

	state Machine {
		attribute log : String = "";

		entry; then S0;
		state S0;
		state S1 {
			entry action { assign log := log + "S1(entry);"; }
			exit action { assign log := log + "S1(exit);"; }
			then A1;
			junction EP;
			history H;
			state A1 { entry action { assign log := log + "A1(entry);"; } }
			state A2 {
				entry action { assign log := log + "A2(entry);"; }
				exit action { assign log := log + "A2(exit);"; }
			}
			transition first EP do assign log := log + "EP(effect);" then A2;
		}
		transition first S0 accept Go do assign log := log + "T1(effect);" then S1::EP;
		transition first S1 accept Back do assign log := log + "T2(effect);" then S0;
		transition first S0 accept Again do assign log := log + "T3(effect);" then S1::H;
	}
}
