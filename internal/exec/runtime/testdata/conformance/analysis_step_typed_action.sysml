// A step of a case body typed by an action definition performs that definition's
// own flow, its inputs bound in the step and its outputs read by the result.
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

	analysis typed {
		subject s = ship;
		action d : Double {
			in v = s.cost;
		}
		return : Real = d.r;
	}
}
