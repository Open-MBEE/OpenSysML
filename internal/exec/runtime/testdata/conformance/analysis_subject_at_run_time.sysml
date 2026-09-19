// An analysis definition binds no subject of its own: the run supplies the object
// the case is about, and its `in` parameter takes its default. The objective is
// evaluated against the result after the body: satisfied here.
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
		in factor : Real default 1.0;
		objective affordable {
			require constraint { total < 20.0 }
		}
		out total : Real = Sum(s.cost, s.other) * factor;
	}

	part ship : Ship;
}
