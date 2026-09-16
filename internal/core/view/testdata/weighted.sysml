package Weighted {
	private import ScalarValues::*;
	private import Stochastic::*;

	action def Route {
		attribute taken : Integer = 0;

		first start;
		decide select;
		action fast { assign taken := 1; }
		action slow { assign taken := 2; }
		done;

		succession first start then select;
		first select then fast { @Probability { p = 0.7; } }
		first select then slow { @Probability { p = 0.3; } }
		succession first fast then done;
		succession first slow then done;
	}
}

package WeightedViews {
	private import Views::*;
	private import StandardViewDefinitions::*;

	view routeView : ActionFlowView {
		expose Weighted::Route;
		render asInterconnectionDiagram;
	}
}
