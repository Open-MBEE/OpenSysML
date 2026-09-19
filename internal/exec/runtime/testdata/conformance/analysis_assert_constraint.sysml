// An `assert constraint` in a case body is a check evaluated in the case's
// frame, reported as a verdict beside the objective's: not satisfied here.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
	}
	part ship : Ship;

	analysis asserted {
		subject s = ship;
		assert constraint cheap { s.cost > 100.0 }
		objective positive {
			require constraint { c > 0.0 }
		}
		out c : Real = s.cost;
	}
}
