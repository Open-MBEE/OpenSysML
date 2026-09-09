// A case one of whose outputs cannot be computed reports the outputs computed
// before it and the evaluations made, but decides nothing: every objective and
// assertion is undecided for the failure, even one whose condition reads only
// an output that was computed, and no evaluation is selected.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 4.0;
		attribute crew : Real = 0.0;
	}
	part ship : Ship;

	analysis def Study {
		subject s : Ship;
		calc rate { in x : Real; return : Real = x + 1.0; }
		in calc f = rate;
		out alpha : Real = f(s.cost);
		out beta : Real = s.cost / s.crew;
		assert constraint alphaPositive { alpha > 0.0 }
		objective cheap {
			require constraint { alpha < 10.0 }
		}
		return : Real = alpha;
	}
	analysis study : Study { subject s = ship; }
}
