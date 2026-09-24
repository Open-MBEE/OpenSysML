// A case computing no output of its own still answers: its objective and its
// asserted constraints are evaluated against the subject and reported as verdicts.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
		attribute limit : Real = 4.0;
	}
	part ship : Ship;

	analysis budget {
		subject s = ship;
		objective affordable {
			require constraint { s.cost <= s.limit }
		}
		assert constraint priced { s.cost > 0.0 }
	}
}
