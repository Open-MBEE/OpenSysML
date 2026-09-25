// An `in` parameter of an analysis usage is bound by a named argument of the run;
// the usage binds its subject itself.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
		attribute other : Real = 7.0;
	}

	analysis def CostAnalysis {
		subject s : Ship;
		in factor : Real default 1.0;
		out total : Real = (s.cost + s.other) * factor;
	}

	part ship : Ship;
	analysis shipCost : CostAnalysis {
		subject s = ship;
	}
}
