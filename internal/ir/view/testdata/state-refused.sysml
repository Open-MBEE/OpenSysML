package Machines {
	alias Rtc for Occurrences::Occurrence::isRunToCompletion;

	// Redefines the library's run-to-completion feature through an alias.
	state def Relaxed {
		attribute :>> Rtc = false;

		entry; then off;

		state off;
		state on;

		transition first off then on;
	}
}

package MachineViews {
	private import StandardViewDefinitions::*;

	view relaxedStates : StateTransitionView {
		expose Machines::Relaxed;
	}
}
