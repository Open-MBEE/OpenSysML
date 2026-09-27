package Instruments {
	action def InitializePEAS;
	action def Warm;
	action def Cool;
	attribute def Reset;
	attribute def Halt;
	attribute def 'Stop, Now';

	// A state's behaviours are named by their own name, their type's when they
	// have none, and by their kind alone when they have neither; the triggers it
	// defers are named too.
	state def PEAS {
		entry; then Init;

		state Init {
			do action initialize : InitializePEAS;
		}
		state Ready {
			entry action : Warm;
			do action;
			exit action cool : Cool;
		}
		state Idle {
			do action;
			exit action wrap { }
			defer Reset, Halt, 'Stop, Now';
		}
		state Booting {
			do action : InitializePEAS;
		}
		transition first Init then Ready;
		transition first Ready then Idle;
		transition first Idle then Booting;
		transition first Booting then done;
	}
}

package InstrumentViews {
	private import StandardViewDefinitions::*;

	view peas : StateTransitionView {
		expose Instruments::PEAS;
	}
}
