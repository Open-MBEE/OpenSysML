package Typed {
	action def Warm;

	action def Cycle;

	// A typed action usage exposed on its own, performing a typed action.
	action run : Cycle {
		first start;
		action warm : Warm;
		done;
		succession first start then warm;
		succession first warm then done;
	}

	state def Heating;

	state def Boiler {
		entry; then idle;

		state idle;
		state heating : Heating;

		transition first idle then heating;
		transition first heating then idle;
	}
}

package TypedViews {
	private import StandardViewDefinitions::*;

	view cycleView : ActionFlowView {
		expose Typed::run;
	}

	view boilerView : StateTransitionView {
		expose Typed::Boiler;
	}
}
