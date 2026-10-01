// A constant feature keeps one value over the lifetime of its featuring
// occurrence (KerML 1.0 §8.3.3.3 Feature::isConstant), so an assignment to
// one once the performance is initialized is refused and the value stays.
package test {
	private import ScalarValues::*;

	action tune {
		constant attribute limit : Integer = 5;

		first start;

		action raise {
			assign limit := 7;
		}

		done;

		succession first start then raise;
		succession first raise then done;
	}
}
