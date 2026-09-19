// An objective's `assume` and `require` constraints both hold over the case's
// result: the verdict is satisfied, reported beside the return value.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
		attribute other : Real = 7.0;
	}
	part ship : Ship;

	analysis def CostAnalysis {
		subject s : Ship;
		in limit : Real;
		objective affordable {
			assume constraint { limit > 0.0 }
			require constraint { result <= limit }
		}
		return : Real = s.cost + s.other;
	}

	analysis shipCost : CostAnalysis { subject s = ship; in limit = 20.0; }
}
