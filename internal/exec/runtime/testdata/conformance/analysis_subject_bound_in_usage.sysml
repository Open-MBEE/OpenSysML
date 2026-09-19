// An analysis case is a calculation (SysML v2 §7.22): its `subject` is an input
// the usage binds, and its `out` parameters are computed in the case's frame
// over the subject's features. The usage's outputs are read as features of it,
// at package level and from a part that owns a case.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
		attribute other : Real = 7.0;
	}

	calc def Sum {
		in a : Real;
		in b : Real;
		return : Real = a + b;
	}

	analysis def CostAnalysis {
		subject s : Ship;
		out total : Real = Sum(s.cost, s.other);
	}

	part ship : Ship;
	analysis shipCost : CostAnalysis {
		subject s = ship;
	}

	// A case binding no subject and stating no steps: its output is a value.
	analysis plain {
		out x : Real = 1.0 + 2.0;
	}

	part def Holder {
		analysis inner : CostAnalysis {
			subject s = h;
		}
		part h : Ship;
	}
	part holder : Holder;

	attribute costRead = shipCost.total;
	attribute plainRead = plain.x;
	attribute innerRead = holder.inner.total;
}
