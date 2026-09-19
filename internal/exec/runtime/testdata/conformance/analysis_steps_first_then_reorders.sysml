// A `first a then b` succession orders the steps of a case body regardless of
// the order they are written in: the step written second runs first, so the
// one written first reads an output the other has already produced.
package test {
	private import ScalarValues::*;

	part def Ship {
		attribute cost : Real = 5.0;
	}
	part ship : Ship;

	analysis reordered {
		subject s = ship;
		action second {
			in q : Real = first1.p;
			out w : Real;
			assign w := q * 10.0;
		}
		action first1 {
			out p : Real;
			assign p := s.cost;
		}
		first first1 then second;
		return : Real = second.w;
	}
}
