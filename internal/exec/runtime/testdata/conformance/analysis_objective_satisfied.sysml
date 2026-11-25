// An objective's `assume` and `require` constraints both hold over the case's
// named result (inside a constraint, `result` is the constraint's own, as the
// pilot resolves it): the verdict is satisfied, reported beside the return value.
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
			require constraint { total <= limit }
		}
		return total : Real = s.cost + s.other;
	}

	analysis shipCost : CostAnalysis { subject s = ship; in limit = 20.0; }
}
