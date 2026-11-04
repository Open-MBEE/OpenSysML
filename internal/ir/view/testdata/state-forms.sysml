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
		succession first running then off;
	}
}

package FurnaceViews {
	private import StandardViewDefinitions::*;

	view burner : StateTransitionView {
		expose Furnace::Burner;
	}

	// the tree draws the def's entry, do and exit members in a `state actions`
	// compartment and its successions in a `successions` compartment.
	view burnerTree : GeneralView {
		expose Furnace::Burner;
	}
	view runningTree : GeneralView {
		expose Furnace::Burner::running;
	}
}
