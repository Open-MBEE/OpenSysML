// A `perform action` in a case body performs the action named, its parameters
// bound by name from the case's values: the case's `v` feeds the action's `in v`
// and the action's `out r` returns to the case's `out r`.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
	}
	part ship : Ship;

	action def Double {
		in v : Real;
		out r : Real;
		first start;
		then action go {
			assign r := v * 2.0;
		}
	}

	analysis performed {
		subject s = ship;
		attribute v : Real = s.cost;
		out r : Real;
		perform action dbl : Double;
		return : Real = r;
	}
}
