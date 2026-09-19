// The body of an analysis case performs its nested actions as steps, `then`
// sequencing them: a later step reads an earlier one's output by `step.pin`,
// and the case's outputs and result read the steps' outputs after the flow.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
	}
	part ship : Ship;

	analysis steps {
		subject s = ship;
		action a {
			out p : Real;
			assign p := s.cost + 1.0;
		}
		then action b {
			in q : Real = a.p;
			out w : Real;
			assign w := q * 10.0;
		}
		out total : Real = b.w + a.p;
		return : Real = b.w;
	}
}
