// A sub-analysis is a step of the case body like an action: `then` sequences it
// after the action whose output it reads, and later steps read its outputs.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
	}
	part ship : Ship;

	analysis def Scale {
		subject s : Ship;
		in factor : Real;
		out scaled : Real = s.cost * factor;
	}

	analysis pipeline {
		subject s = ship;
		action pick {
			out f : Real;
			assign f := 3.0;
		}
		then analysis scale : Scale { in factor = pick.f; }
		then action finish {
			in v : Real = scale.scaled;
			out w : Real;
			assign w := v + 1.0;
		}
		out total : Real = finish.w;
	}
}
