package Instruments {
	action def InitializePEAS;
	action def Warm;
	action def Cool;
	attribute def Reset;

	// A state's behaviours are named by their own name, their type's when they
	// have none, and by their kind alone when they have neither.
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
		}
		transition first Init then Ready;
		transition first Ready then Idle;
		transition first Idle then done;
	}
}

package InstrumentViews {
	private import StandardViewDefinitions::*;

	view peas : StateTransitionView {
		expose Instruments::PEAS;
	}
}
