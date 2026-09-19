// An `in` parameter of an analysis case is bound by a positional argument of the
// run, after the subject. The objective reads the result the argument scaled:
// not satisfied here.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
		attribute other : Real = 7.0;
	}

	analysis def CostAnalysis {
		subject s : Ship;
		in factor : Real default 1.0;
		objective affordable {
			require constraint { total < 20.0 }
		}
		out total : Real = (s.cost + s.other) * factor;
	}

	part ship : Ship;
}
