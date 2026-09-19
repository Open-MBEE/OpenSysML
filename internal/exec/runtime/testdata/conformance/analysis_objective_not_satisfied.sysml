// An objective is evaluated after the body: its `require` constraint is checked
// against the case's outputs, and a bound it exceeds is reported not satisfied
// beside the outputs rather than failing the run.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
		attribute other : Real = 7.0;
	}
	part ship : Ship;

	calc def Sum { in a : Real; in b : Real; return : Real = a + b; }

	analysis budget {
		subject s = ship;
		attribute limit : Real = 10.0;
		objective affordable {
			require constraint { total <= limit }
		}
		out total : Real = Sum(s.cost, s.other);
	}
}
