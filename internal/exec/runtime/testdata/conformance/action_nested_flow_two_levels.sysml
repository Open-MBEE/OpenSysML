package test {
	private import ScalarValues::*;

	// Nesting is not limited to one level: an inner node states a flow of its own
	// the same way, and each completes only when its own flow does.
	action trek {
		out attribute order : String;

		first leg;

		action leg {
			first inner;
			action inner {
				action deepest { assign order := order + "c"; }
				first deepest;
				succession first deepest then innerDone;
				action innerDone { assign order := order + "b"; }
			}
			succession first inner then legDone;
			action legDone { assign order := order + "a"; }
		}

		attribute order : String = "";
		succession first leg then done;
		done;
	}
}
