package Furnace {
	private import ScalarValues::Boolean;
	action def Ignite;
	action def Purge;

	// A composite state draws its entry, do and exit actions as nodes of its
	// state-transition view; a parallel state writes «parallel» under its name.
	state def Burner {
		attribute flame : Boolean;
		entry; then off;

		state off;
		state running parallel {
			entry action ignite : Ignite;
			exit action : Purge;
			do action { assign flame := true; }
			state low;
			state high;
		}
		transition first off then running;
	}
}

package FurnaceViews {
	private import StandardViewDefinitions::*;

	view burner : StateTransitionView {
		expose Furnace::Burner;
	}
}
